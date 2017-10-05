package main

import (
	"errors"
	"flag"
	"fmt"
	. "github.com/stellar/go/sampler"
	"github.com/stellar/go/xdr"
	"github.com/stellar/horizon/db2/core"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"time"
)

const Debug bool = true

func samplerLoop(postgresqlConnection, stellarCoreUrl string, cancellation <-chan struct{}, txRate uint32) {
	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(10 * time.Second)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, stellarCoreUrl, postgresqlConnection)
	var sampler TransactionGenerator = NewTransactionGenerator()
	var database Database = NewInMemoryDatabase(sequenceFetcher)

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	ticker := time.NewTicker(time.Second)
	for {
		for it := uint32(0); it < txRate; it++ {
			txError := singleTransaction(database, submitter, sampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
				return
			}
		}
		select {
		case <-cancellation:
			return
		case <-ticker.C:
		}
	}
}

func singleTransaction(database Database, submitter TxSubmitter, sampler TransactionGenerator) error {
	database.BeginTransaction()
	defer database.EndTransaction()

	Logger.Printf("sampling data")
	data, sourceAccount := sampler(1, database)
	Logger.Printf("data sampled %+v", &data)
	if data == nil || sourceAccount == nil {
		Logger.Printf("unable to generate correct transaction, continuing...")
		return errors.New("unable to generate correct transaction")
	}

	Logger.Print("submitting tx")
	submitResult, sequenceUpdate, transactionResult := submitter.Submit(sourceAccount, data)
	if submitResult.Err != nil {
		Logger.Printf("tx submit rejected: %s", submitResult.Err)
		handleTransactionError(database, transactionResult)

		return errors.New("tx submit rejected")
	}
	Logger.Print("tx submitted")
	return nil

	Logger.Print("waiting for tx to externalize (seqnum increase)")
	newSequenceNum, seqError := sequenceUpdate()
	if seqError != nil {
		Logger.Print("error while checking if tx was externalized; downloading tx result")
		handleTransactionError(database, transactionResult)

		return errors.New("error while checking if tx was externalized")
	}
	sourceAccount.SeqNum = xdr.SequenceNumber(newSequenceNum.Sequence)

	if Debug {
		txResult, txError := transactionResult()
		if txError != nil {
			Logger.Printf("error while downloading tx result: %+v", txError)
		} else {
			Logger.Printf("tx result: %+v", txResult)
		}
	}

	Logger.Print("tx externalized, finishing")
	return nil
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

func main() {
	// test()
	// return

	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
	stellarCoreURL := flag.String("core", "http://localhost:11626", "stellar-core http endpoint's url")
	flag.Parse()

	var cancellation chan struct{} = make(chan struct{}, 2)
	defer func() {
		cancellation <- struct{}{}
		close(cancellation)
	}()

	setupSignalHandler(cancellation)

	samplerLoop(*postgresConnectionString, *stellarCoreURL, cancellation, 50)
}
