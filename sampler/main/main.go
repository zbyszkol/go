package main

import (
	"flag"
	"fmt"
	. "github.com/stellar/go/sampler"
	"github.com/stellar/go/xdr"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func SamplerLoop(postgresqlConnection, stellarCoreUrl string, cancellation <-chan struct{}) {
	var sampler TransactionGenerator = NewTransactionGenerator()
	var database Database = NewInMemoryDatabase()

	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(time.Second)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, stellarCoreUrl, postgresqlConnection)

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	for {
		select {
		case <-cancellation:
			return
		default:
		}

		database.BeginTransaction()

		size := uint64(rand.Intn(100) + 1)
		Logger.Printf("sampling data")
		data, sourceAccount := sampler(size, database)
		Logger.Printf("data sampled %+v", &data)
		if data == nil || sourceAccount == nil {
			Logger.Printf("unable to generate correct transaction, exiting...")
			return
		}

		Logger.Print("submitting tx")
		submitResult, seqenceUpdate, transactionResult := submitter.Submit(sourceAccount, data)
		if submitResult.Err != nil {
			// TODO scream and run away, don't forget to take your dog
			Logger.Printf("tx submit rejected: %s", submitResult.Err)
			database.RejectTransaction()
			continue
		}
		Logger.Print("tx submitted")

		Logger.Print("waiting for tx to externalize (seqnum increase)")
		newSequenceNum, seqError := seqenceUpdate()
		if seqError != nil {
			// TODO clean up - there are at least two sampler.go files
			Logger.Print("error while checking if tx was externalized; downloading tx result")
			coreResult, transError := transactionResult()
			if transError != nil {
				Logger.Print("that's it, rejecting")
				database.RejectTransaction()
			} else {
				Logger.Print("applying changes")
				database = ApplyChanges(&coreResult.ResultMeta, database)
			}
			continue
		}
		sourceAccount.SeqNum = xdr.SequenceNumber(newSequenceNum.Sequence)

		Logger.Print("tx externalized, finishing")

		database.EndTransaction()
	}
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
	partition := GetRandomPartitionWithoutZeros(int64(value), size)
	fmt.Println(partition)
}

func main() {
	test()
	return
	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
	stellarCoreUrl := flag.String("core", "http://localhost:11626", "stellar-core http endpoint's url")
	flag.Parse()

	var cancellation chan struct{} = make(chan struct{}, 2)
	defer func() {
		cancellation <- struct{}{}
		close(cancellation)
	}()

	setupSignalHandler(cancellation)

	SamplerLoop(*postgresConnectionString, *stellarCoreUrl, cancellation)
}
