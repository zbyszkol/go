package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/stellar/go/build"
	. "github.com/stellar/go/sampler"
	. "github.com/stellar/go/sampler/failureDetector"
	"math"
	"net/http"
	"time"
)

const Debug bool = true

type CommitHelper struct {
	waitCommitQueue []CommitResult
	commitQueue     []CommitResult
}

func NewCommitHelper() CommitHelper {
	return CommitHelper{waitCommitQueue: []CommitResult{}, commitQueue: []CommitResult{}}
}

func singleTransaction(database Database, submitter TxSubmitter, sampler TransactionGenerator) (uint, func() (*build.Sequence, error), CommitResult, error) {
	var operationsCount uint = 0
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

		return 0, nil, nil, errors.New("tx submit rejected")
	}
	Logger.Print("tx submitted")

	operationsCount = uint(len(data.TX.Operations))
	return operationsCount, sequenceUpdate, commitTransaction, nil
}

func (helper *CommitHelper) AddToCommitQueue(commit CommitResult) {
	helper.waitCommitQueue = append(helper.waitCommitQueue, commit)
}

func (helper *CommitHelper) ProcessCommitQueue(database Database) Database {
	Logger.Print("processing the commit queue")

	for _, commit := range helper.commitQueue {
		database = commit(database)
	}
	tmp := helper.commitQueue
	helper.commitQueue = helper.waitCommitQueue
	helper.waitCommitQueue = tmp[0:0]

	Logger.Println("commit queue processed")
	return database
}

func failureDetector(postgresConnectionString string, ledgerNumber uint64) {
	coreDb := NewDbSession(postgresConnectionString)
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

	var database Database = NewInMemoryDatabase(sequenceFetcher)
	localSampler := NewCommitHelper()

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	Logger.Println("Starting benchmark's accounts creation procedure")

	database = InitializeAccounts(submitter, database, uint64(expectedNumberOfAccounts), 50)

	Logger.Println("Accounts creation procedure finished")

	paymentGenerator := GeneratorsListEntry{Generator: GetValidPaymentMutatorNative, Bias: 100}
	paymentSampler := NewTransactionGenerator(paymentGenerator)

	ticker := time.NewTicker(time.Second)

	for {

		database = localSampler.ProcessCommitQueue(database)

		collisionLimit := uint32(math.Sqrt(float64(database.GetAccountsCount())))
		Logger.Printf("Collision limit is %d", collisionLimit)
		for it := uint32(0); it < txRate && it < collisionLimit; it++ {
			_, _, commitTx, txError := singleTransaction(database, submitter, paymentSampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
				continue
			}

			localSampler.AddToCommitQueue(commitTx)
		}
		Logger.Print("transactions generated with the specified txRate")
		<-ticker.C
	}
}

func main() {
	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
	stellarCoreURL := flag.String("core", "http://localhost:11626", "stellar-core http endpoint's url")
	failure_flag := "failure_detector"
	ledgerNumber := flag.Uint64(failure_flag, 0, "max ledger number for failure searching procedure")

	flag.Parse()

	if *ledgerNumber > 0 {
		failureDetector(*postgresConnectionString, *ledgerNumber)
		return
	}

	const txRate, expectedNumberOfAccounts uint32 = 10000, 1000000

	httpClient := http.DefaultClient
	httpClient.Timeout = time.Duration(10 * time.Minute)
	submitter, accountFetcher, sequenceFetcher := NewTxSubmitter(httpClient, *stellarCoreURL, *postgresConnectionString)
	benchmarkScenario(httpClient, submitter, accountFetcher, sequenceFetcher, txRate, expectedNumberOfAccounts)
}
