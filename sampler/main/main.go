package main

import (
	// "errors"
	"flag"
	"fmt"
	"github.com/stellar/go/build"
	. "github.com/stellar/go/sampler"
	. "github.com/stellar/go/sampler/failureDetector"
	"github.com/stellar/horizon/db2/core"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"time"
)

const Debug bool = true

func samplerLoop(postgresqlConnection, stellarCoreUrl string, cancellation <-chan struct{}, txRate, numberOfOperations, expectedNumberOfAccounts uint) {
	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(10 * time.Second)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, stellarCoreUrl, postgresqlConnection)
	// var sampler TransactionGenerator = DefaultTransactionGenerator()

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
	var operationsCounter uint = 0
	for operationsCounter < numberOfOperations {
		localSampler.processCommitQueue()
		for it := uint(0); it < txRate; it++ {
			operationsCount, txError := localSampler.singleTransaction(submitter, sampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
				panic("error while committing a transaction")
			} else {
				operationsCounter += operationsCount
			}
		}
		Logger.Print("transactions generated with the specified txRate")
		select {
		case <-cancellation:
			return
		case <-ticker.C:
		}
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

	data, sourceAccount, commitTransaction := sampler(1, database)

	Logger.Printf("data sampled %+v", &data)
	if data == nil || sourceAccount == nil {
		Logger.Printf("unable to generate correct transaction, continuing...")
		return 0, nil // errors.New("unable to generate correct transaction")
	}
	operationsCount = uint(len(data.TX.Operations))
	Logger.Print("submitting tx")
	submitResult, sequenceUpdate, transactionResult := submitter.Submit(sourceAccount, data)
	if submitResult.Err != nil {
		Logger.Printf("tx submit rejected: %s", submitResult.Err)
		database = handleTransactionError(database, transactionResult)

		return operationsCount, nil // errors.New("tx submit rejected")
	}
	Logger.Print("tx submitted")
	// TODO move it somewhere else. Commit after you're sure that tx was externalized.
	// database = commitTransaction(database)

	// return operationsCount, nil
	this.addToCommitQueue(sequenceUpdate, commitTransaction)

	// Logger.Print("waiting for tx to externalize (seqnum increase)")
	// newSequenceNum, seqError := sequenceUpdate()
	// if seqError != nil {
	// 	Logger.Print("error while checking if tx was externalized; downloading tx result")
	// 	database = handleTransactionError(database, transactionResult)

	// 	return operationsCount, errors.New("error while checking if tx was externalized")
	// }
	// sourceAccount.SeqNum = xdr.SequenceNumber(newSequenceNum.Sequence)

	// if Debug {
	// 	// txResult, txError := transactionResult()
	// 	// if txError != nil {
	// 	// 	Logger.Printf("error while downloading tx result: %+v", txError)
	// 	// } else {
	// 	// 	Logger.Printf("tx result: %+v", txResult)
	// 	// }
	// 	database = handleTransactionError(database, transactionResult)
	// }

	return operationsCount, nil
}

func (sampler *commitHelper) addToCommitQueue(confirm func() (*build.Sequence, error), commit func(Database) Database) {
	sampler.counter++
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

func setupSignalHandler(cancellationChannel chan struct{}) {
	return
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, os.Interrupt, os.Kill)
	go func() {
		for range osSignal {
			cancellationChannel <- struct{}{}
		}
	}()
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

func main() {
	// test()
	// return

	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
	stellarCoreURL := flag.String("core", "http://localhost:11626", "stellar-core http endpoint's url")
	flag.Parse()

	// failureDetector(*postgresConnectionString)
	// return

	var cancellation chan struct{} = make(chan struct{}, 2)
	defer func() {
		cancellation <- struct{}{}
		close(cancellation)
	}()

	setupSignalHandler(cancellation)

	const txRate, numberOfOperations, expectedNumberOfAccounts uint = 100, 1000000, 10000

	samplerLoop(*postgresConnectionString, *stellarCoreURL, cancellation, txRate, numberOfOperations, expectedNumberOfAccounts)
}
