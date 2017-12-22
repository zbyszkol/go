package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/stellar/go/build"
	. "github.com/stellar/go/sampler"
	. "github.com/stellar/go/sampler/failureDetector"
	"github.com/stellar/horizon/db2/core"
	"math"
	"net/http"
	"time"
)

const Debug bool = true

type CommitHelper struct {
	waitCommitQueue []CommitResult
	commitQueue     []CommitResult
	database        Database
}

func NewCommitHelper(database Database) CommitHelper {
	return CommitHelper{waitCommitQueue: []CommitResult{}, commitQueue: []CommitResult{}, database: database}
}

func singleTransaction(database Database, submitter TxSubmitter, sampler TransactionGenerator) (uint, func() (*build.Sequence, error), CommitResult, error) {
	var operationsCount uint = 0
	// TODO remove this
	database.BeginTransaction()
	defer database.EndTransaction()

	Logger.Printf("sampling data")

	data, sourceAccount, commitTransaction, rejectTransaction := sampler(1, database)

	Logger.Printf("data sampled %+v", &data)
	if data == nil || sourceAccount == nil {
		Logger.Printf("unable to generate a correct transaction, continuing...")
		return 0, nil, nil, errors.New("unable to generate correct transaction")
	}
	Logger.Print("submitting tx")
	submitResult, sequenceUpdate, _ := submitter.Submit(sourceAccount, data)
	if submitResult.Err != nil {
		Logger.Printf("tx submit rejected: %s", submitResult.Err)
		database = rejectTransaction(database)
		// panic("tx submit rejected")

		return 0, nil, nil, errors.New("tx submit rejected")
	}
	Logger.Print("tx submitted")

	operationsCount = uint(len(data.TX.Operations))
	return operationsCount, sequenceUpdate, commitTransaction, nil
}

func (helper *CommitHelper) AddToCommitQueue(confirm func() (*build.Sequence, error), commit CommitResult) {
	helper.waitCommitQueue = append(helper.waitCommitQueue, commit)
}

func (helper *CommitHelper) ProcessCommitQueue() {
	Logger.Print("processing the commit queue")

	for _, commit := range helper.commitQueue {
		helper.database = commit(helper.database)
	}
	tmp := helper.commitQueue
	helper.commitQueue = helper.waitCommitQueue
	helper.waitCommitQueue = tmp[0:0]

	Logger.Println("commit queue processed")
}

func handleTransactionError(database Database, transactionResult func() (*core.Transaction, error)) Database {
	database.RejectTransaction()
	coreResult, transError := transactionResult()
	Logger.Printf("core result: %+v", coreResult)
	if transError != nil {
		Logger.Print("that's it, rejecting")
	} else {
		Logger.Print("applying changes")
		if coreResult != nil {
			if !coreResult.IsSuccessful() {
				Logger.Printf("transaction was failed")
				panic("transaction was failed")
			}
			database = ApplyChanges(&coreResult.ResultMeta, database)
		} else {
			Logger.Printf("coreResult is nil")
		}
	}
	return database
}

func failureDetector(postgresConnectionString string, ledgerNumber uint32) {
	coreDb := NewDbSession(postgresConnectionString)
	// TODO move this magic number as a parameter
	iterator := FindFailedTransactions(coreDb, ledgerNumber)
	noError := true
	for hasNext, error := iterator.Next(); hasNext; hasNext, error = iterator.Next() {
		noError = false
		if error != nil {
			Logger.Printf("error while iterating transactions: %s", error)
		}
		tx := iterator.Get()
		fmt.Println("------------------------")
		PrintFailuresFromCoreTx(tx)
		fmt.Println()
		fmt.Println("------------------------")
	}
	if noError {
		fmt.Println("no tx error")
	}
}

func benchmarkScenario(
	httpClient *http.Client,
	submitter TxSubmitter,
	accountFetcher AccountFetcher,
	sequenceFetcher SequenceNumberFetcher,
	txRate,
	expectedNumberOfAccounts uint32) {

	// TODO commented due to new accounts generation mechanism
	// accountGenerator := GeneratorsListEntry{Generator: GetValidCreateAccountMutator, Bias: 100}
	// var accountSampler TransactionGenerator = NewTransactionGenerator(accountGenerator)

	var database Database = NewInMemoryDatabase(sequenceFetcher)
	localSampler := NewCommitHelper(database)

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	// create some amount of accounts
	Logger.Println("Starting benchmark's accounts creation procedure")

	database = InitializeAccounts(submitter, database.GetAccountByOrder(0), database, uint64(expectedNumberOfAccounts), 100)

	// for accountsCount, createdAccounts := uint(0), uint(0); accountsCount < expectedNumberOfAccounts; accountsCount += createdAccounts {

	// 	localSampler.processCommitQueue()

	// 	createdAccounts = 0
	// 	collisionLimit := uint(math.Sqrt(float64(localSampler.database.GetAccountsCount())))
	// 	for it := uint(0); it < txRate && it < collisionLimit; it++ {
	// 		accountsNumber, sequenceUpdate, commitTx, txError := singleTransaction(database, submitter, accountSampler)
	// 		if txError != nil {
	// 			Logger.Printf("error while committing a transaction: %s", txError)
	// 			continue
	// 		}
	// 		createdAccounts += accountsNumber

	// 		localSampler.addToCommitQueue(sequenceUpdate, commitTx)
	// 	}
	// 	Logger.Printf("Collision limit was %d", collisionLimit)
	// 	Logger.Printf("Created %d new accounts", createdAccounts)

	// 	<-time.NewTicker(time.Second).C
	// }

	Logger.Println("Accounts creation procedure finished")

	paymentGenerator := GeneratorsListEntry{Generator: GetValidPaymentMutatorNative, Bias: 100}
	paymentSampler := NewTransactionGenerator(paymentGenerator)

	ticker := time.NewTicker(time.Second)

	for {

		localSampler.ProcessCommitQueue()

		collisionLimit := uint32(math.Sqrt(float64(localSampler.database.GetAccountsCount())))
		Logger.Printf("Collision limit is %d", collisionLimit)
		for it := uint32(0); it < txRate && it < collisionLimit; it++ {
			_, sequenceUpdate, commitTx, txError := singleTransaction(database, submitter, paymentSampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
				continue
			}

			localSampler.AddToCommitQueue(sequenceUpdate, commitTx)
		}
		Logger.Print("transactions generated with the specified txRate")
		<-ticker.C
	}
}

func main() {
	// test()
	// return

	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
	stellarCoreURL := flag.String("core", "http://localhost:11626", "stellar-core http endpoint's url")
	flag.Parse()

	// failureDetector(*postgresConnectionString, 1000)
	// return

	const txRate, expectedNumberOfAccounts uint32 = 1000, 1000000

	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(10 * time.Minute)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, *stellarCoreURL, *postgresConnectionString)
	benchmarkScenario(httpClient, submitter, accountFetcher, sequenceFetcher, txRate, expectedNumberOfAccounts)

	// TODO write looper which just generates transactions and write them into a file/output then play them back to stellar-core
}
