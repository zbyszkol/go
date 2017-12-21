package main

import (
	"github.com/stellar/go/amount"
	"github.com/stellar/go/build"
	. "github.com/stellar/go/sampler"
	"github.com/stellar/go/xdr"
	"time"
)

const txMaxSize uint64 = 100

// For each created account there will be exactly one transaction creating 1-100 new accounts.
// Keep in mind the max_tx_set_size.
func InitializeAccounts(submitter TxSubmitter, rootAccount *AccountEntry, database Database, numberOfAccounts uint64, txRate uint32) Database {
	var balancePerAccount uint64 = uint64(rootAccount.Balance) / numberOfAccounts
	accountIx := 0
	accountsPerTx := txMaxSize
	commitOperations := []CommitResult{}
	for accountsLeft := numberOfAccounts; accountsLeft > 0; {

		for nTx := uint32(0); nTx < txRate && accountsLeft > 0 && accountIx < database.GetAccountsCount(); accountIx++ {
			if accountsPerTx > accountsLeft {
				accountsPerTx = accountsLeft
			}
			sourceAccount := database.GetAccountByOrder(accountIx)
			txBuilder, commitResult, newAccounts := buildCreateAccountTx(sourceAccount, balancePerAccount, accountsPerTx)
			submitResult, _, _ := submitter.Submit(sourceAccount, txBuilder)
			if submitResult.Err != nil {
				panic("tx rejected")
			}
			commitOperations = append(commitOperations, commitResult)
			accountsLeft -= newAccounts
		}
		<-time.NewTicker(10 * time.Second).C
		for _, commit := range commitOperations {
			database = commit(database)
		}
		commitOperations = commitOperations[0:0]
	}
	return database
}

func buildCreateAccountTx(sourceAccount *AccountEntry, amountLeft, accountsPerTx uint64) (*build.TransactionBuilder, CommitResult, uint64) {

	var startingBalance int64 = int64((int64(sourceAccount.Balance) - int64(int64(amountLeft)+BaseFee)) / int64(accountsPerTx))
	sourceAccount.Balance -= xdr.Int64(int64(uint64(startingBalance) * accountsPerTx))
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
	for ix := uint64(0); ix < accountsPerTx; ix++ {
		newAccount := CreateNewAccount()
		destination := build.Destination{newAccount.Keypair.GetSeed().Address()}
		amount := build.NativeAmount{amount.String(xdr.Int64(startingBalance))}
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
		accountsPerTx
}
