package sampler

import (
	"github.com/stellar/go/xdr"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func SamplerLoop() {
	var sampler TransactionGenerator = NewTransactionGenerator()
	var database Database = NewInMemoryDatabase()

	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(time.Second)
	var submitter TxSubmitter
	var accountFetcher AccountFetcher
	var sequenceFetcher SequenceNumberFetcher
	submitter, accountFetcher, sequenceFetcher = NewTxSubmitter(httpClient, "http://localhost:11626", "postgresql://dbname=core host=localhost user=stellar password=__PGPASS__")

	var cancellation chan struct{} = make(chan struct{}, 2)
	defer close(cancellation)

	setupSignalHandler(cancellation)

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

		size := Size(rand.Intn(100) + 1)
		data, sourceAccount := sampler(size, database)

		submitResult, seqenceUpdate, transactionResult := submitter.Submit(sourceAccount, data)
		if submitResult.Err != nil {
			// TODO scream and run away, don't forget to take your dog
			database.RejectTransaction()
			continue
		}

		newSequenceNum, seqError := seqenceUpdate()
		if seqError != nil {
			// TODO clean up - there are at least two sampler.go files
			coreResult, transError := transactionResult()
			if transError != nil {
				database.RejectTransaction()
			} else {
				database = ApplyChanges(&coreResult.ResultMeta, database)
			}
			continue
		}
		sourceAccount.SeqNum = xdr.SequenceNumber(newSequenceNum.Sequence)

		database.EndTransaction()
	}
}

func setupSignalHandler(cancellationChannel chan struct{}) {
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, os.Interrupt, os.Kill)
	go func() {
		for range osSignal {
			cancellationChannel <- struct{}{}
		}
	}()
}
