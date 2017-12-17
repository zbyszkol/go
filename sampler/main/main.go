package main

import (
	"flag"
	"fmt"
	"github.com/stellar/go/build"
	. "github.com/stellar/go/sampler"
	. "github.com/stellar/go/sampler/failureDetector"
	"github.com/stellar/horizon/db2/core"
	"math/rand"
	"net/http"
	"sort"
	"time"
)

const Debug bool = true

// TODO modify this so one can use it in a benchmark
func samplerLoop(postgresqlConnection, stellarCoreUrl string, txRate, numberOfOperations, expectedNumberOfAccounts uint) {
	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(10 * time.Second)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, stellarCoreUrl, postgresqlConnection)

	var accountProbability = uint((float64(expectedNumberOfAccounts) / float64(numberOfOperations)) * 100)
	Logger.Printf("account's probability: %d", accountProbability)
	accountGenerator := GeneratorsListEntry{Generator: GetValidCreateAccountMutator, Bias: accountProbability}
	paymentGenerator := GeneratorsListEntry{Generator: GetValidPaymentMutatorNative, Bias: 100 - accountProbability}
	var sampler TransactionGenerator = NewTransactionGenerator(accountGenerator, paymentGenerator)

	var database Database = NewInMemoryDatabase(sequenceFetcher)

	localSampler := newCommitHelper(database)

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	ticker := time.NewTicker(time.Second)
	for operationsCounter, operationsCount := uint(0), uint(0); operationsCounter < numberOfOperations; operationsCounter += operationsCount {
		localSampler.processCommitQueue()
		for it := uint(0); it < txRate; it++ {
			var txError error = nil
			operationsCount, txError = localSampler.singleTransaction(submitter, sampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
				panic("error while committing a transaction")
			}
		}
		Logger.Print("transactions generated with the specified txRate")
		<-ticker.C
	}
}

type txPair struct {
	confirm func() (*build.Sequence, error)
	commit  func(Database) Database
}

func newCommitHelper(database Database) commitHelper {
	return commitHelper{counter: 0, commitQueue: make(map[uint64]txPair), database: database}
}

type commitHelper struct {
	counter     uint64
	commitQueue map[uint64]txPair
	database    Database
}

func (this *commitHelper) singleTransaction(submitter TxSubmitter, sampler TransactionGenerator) (uint, error) {
	database := this.database
	var operationsCount uint = 0
	database.BeginTransaction()
	defer database.EndTransaction()

	Logger.Printf("sampling data")

	data, sourceAccount, commitTransaction, rejectTransaction := sampler(1, database)

	Logger.Printf("data sampled %+v", &data)
	if data == nil || sourceAccount == nil {
		Logger.Printf("unable to generate a correct transaction, continuing...")
		return 0, nil // errors.New("unable to generate correct transaction")
	}
	Logger.Print("submitting tx")
	submitResult, sequenceUpdate, _ := submitter.Submit(sourceAccount, data)
	if submitResult.Err != nil {
		Logger.Printf("tx submit rejected: %s", submitResult.Err)
		database = rejectTransaction(database)
		// panic("tx submit rejected")

		return 0, nil // errors.New("tx submit rejected")
	}
	Logger.Print("tx submitted")

	this.addToCommitQueue(sequenceUpdate, commitTransaction)

	operationsCount = uint(len(data.TX.Operations))
	return operationsCount, nil
}

func (sampler *commitHelper) addToCommitQueue(confirm func() (*build.Sequence, error), commit func(Database) Database) {
	sampler.counter++
	// TODO change me!
	// confirm = func() (*build.Sequence, error) {
	// 	return &build.Sequence{}, nil
	// }
	sampler.commitQueue[sampler.counter] = txPair{confirm, commit}
}

func (sampler *commitHelper) processCommitQueue() {
	Logger.Print("processing the commit queue")
	for key, value := range sampler.commitQueue {
		newSequence, seqError := value.confirm()
		if seqError != nil {
			Logger.Print("error while checking if tx was externalized; downloading tx result")
			panic("error while checking if tx was externalized")
		}
		if newSequence == nil {
			continue
		}
		sampler.database = value.commit(sampler.database)
		Logger.Print("tx externalized, finishing")
		delete(sampler.commitQueue, key)
	}
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

func test() {
	value := rand.Intn(100) + 1
	size := rand.Intn(value) + 1
	partition := GetRandomPartitionWithZeros(int64(value), size)
	fmt.Println(partition)

	testData := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	testData = GetUniformMofNFromSlice(testData, 4)
	sort.Ints(testData)
	fmt.Println(testData)
}

func failureDetector(postgresConnectionString string) {
	coreDb := NewDbSession(postgresConnectionString)
	// TODO move this magic number as a parameter
	iterator := FindFailedTransactions(coreDb, 700)
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
	numberOfOperations,
	expectedNumberOfAccounts uint) {

	accountGenerator := GeneratorsListEntry{Generator: GetValidCreateAccountMutator, Bias: 100}
	paymentGenerator := GeneratorsListEntry{Generator: GetValidPaymentMutatorNative, Bias: 100 - accountGenerator.Bias}
	var accountSampler TransactionGenerator = NewTransactionGenerator(accountGenerator)

	var database Database = NewInMemoryDatabase(sequenceFetcher)
	localSampler := newCommitHelper(database)

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	// create some amount of accounts
	for operationsCounter, operationsCount := uint(0), uint(0); operationsCounter < expectedNumberOfAccounts; operationsCounter += operationsCount {
		localSampler.processCommitQueue()
		var txError error = nil
		operationsCount, txError = localSampler.singleTransaction(submitter, accountSampler)
		if txError != nil {
			Logger.Printf("error while committing a transaction: %s", txError)
			panic("error while committing a transaction")
		}
	}
	Logger.Println("Accounts creation procedure finished")

	paymentGenerator.Bias = 100
	sampler := NewTransactionGenerator(paymentGenerator)

	ticker := time.NewTicker(time.Second)
	for operationsCounter, operationsCount := uint(0), uint(0); operationsCounter < numberOfOperations; operationsCounter += operationsCount {
		localSampler.processCommitQueue()
		for it := uint(0); it < txRate; it++ {
			var txError error = nil
			operationsCount, txError = localSampler.singleTransaction(submitter, sampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
				panic("error while committing a transaction")
			}
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

	// failureDetector(*postgresConnectionString)
	// return

	const txRate, numberOfOperations, expectedNumberOfAccounts uint = 1000, 10000000, 1000000

	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(10 * time.Second)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, *stellarCoreURL, *postgresConnectionString)
	benchmarkScenario(httpClient, submitter, accountFetcher, sequenceFetcher, txRate, numberOfOperations, expectedNumberOfAccounts)

	// TODO write looper which just generates transactions and write them into a file/output then play them back to stellar-core
}

// TODO new version of the sampler:
// 1) generate some number of accounts with same startingBalance
// 2) pause
// 3) use them for generating payment transactions
