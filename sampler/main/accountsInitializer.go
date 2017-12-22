package main

import (
	"github.com/stellar/go/amount"
	"github.com/stellar/go/build"
	. "github.com/stellar/go/sampler"
	"github.com/stellar/go/xdr"
	"time"
)

const txMaxSize uint64 = 100

// For each newly created account there will be exactly one transaction creating 100 new accounts.
func InitializeAccounts(submitter TxSubmitter, database Database, numberOfAccounts uint64, txRate uint32) Database {
	accountIx := database.GetAccountsCount() - 1
	rootAccount := database.GetAccountByOrder(accountIx)
	var balancePerAccount uint64 = uint64(rootAccount.Balance) / (numberOfAccounts + 1)
	accountsPerTx := txMaxSize
	commitHelper := NewCommitHelper()
	for accountsLeft := numberOfAccounts; accountsLeft > 0; {

		database = commitHelper.ProcessCommitQueue(database)

		for nTx := uint32(0); nTx < txRate && accountsLeft > 0 && accountIx < database.GetAccountsCount(); accountIx, nTx = accountIx+1, nTx+1 {
			sourceAccount := database.GetAccountByOrder(accountIx)
			if accountsPerTx > accountsLeft {
				accountsPerTx = accountsLeft
			}
			database.BeginTransaction()

			txBuilder, commitResult, rejectTx, newAccounts := buildCreateAccountTx(sourceAccount, balancePerAccount, accountsPerTx)
			submitResult, _, _ := submitter.Submit(sourceAccount, txBuilder)
			if submitResult.Err != nil {
				Logger.Printf("tx rejected: %s", submitResult.Err)
				database = rejectTx(database)
				continue
			}
			commitHelper.AddToCommitQueue(nil, commitResult)
			accountsLeft -= newAccounts

			database.EndTransaction()
		}
		Logger.Println("Round finished, waiting for next round...")
		<-time.NewTicker(10 * time.Second).C
	}
	return database
}

func buildCreateAccountTx(sourceAccount *AccountEntry, amountLeft, accountsPerTx uint64) (*build.TransactionBuilder, CommitResult, RejectResult, uint64) {

	var startingBalance int64 = (int64(sourceAccount.Balance) - int64(amountLeft+accountsPerTx*uint64(BaseFee))) / int64(accountsPerTx)
	initialBalance := sourceAccount.Balance
	sourceAccount.Balance -= xdr.Int64(uint64(startingBalance)*accountsPerTx + accountsPerTx*uint64(BaseFee))
	seq, seqError := sourceAccount.GetSequence()
	if seqError != nil {
		Logger.Print("error while getting account's sequence number")
		panic("Error while getting account't sequence number")
	}
	seq.Sequence++
	sourceAccount.SetSequence(seq)
	operations := []build.TransactionMutator{
		build.SourceAccount{sourceAccount.Keypair.GetSeed().Address()},
		seq,
		build.TestNetwork,
	}
	commitOperations := []CommitResult{}
	amount := build.NativeAmount{amount.String(xdr.Int64(startingBalance))}
	for ix := uint64(0); ix < accountsPerTx; ix++ {
		newAccount := CreateNewAccount()
		destination := build.Destination{newAccount.Keypair.GetSeed().Address()}
		result := build.CreateAccount(destination, amount)
		operations = append(operations, result)
		commitOperation :=
			func(database Database) Database {
				newAccount.Balance = xdr.Int64(startingBalance)
				database.AddAccount(&newAccount)
				return database
			}
		commitOperations = append(commitOperations, commitOperation)
	}
	transaction := build.Transaction(
		operations...,
	)
	return transaction,
		func(database Database) Database {
			for _, operation := range commitOperations {
				database = operation(database)
			}
			return database
		},
		func(database Database) Database {
			sourceAccount.Balance = initialBalance - xdr.Int64(int64(accountsPerTx)*BaseFee)
			return database
		},
		accountsPerTx
}
