package main

import (
	"github.com/stellar/horizon/txsub"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	. "github.com/stellar/go/sampler"
	. "github.com/stellar/go/sampler/failureDetector"
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

	database, rootError := AddRootAccount(database, accountFetcher, sequenceFetcher)
	if rootError != nil {
		panic("Unable to add the root account.")
	}

	ticker := time.NewTicker(time.Second)
	var operationsCounter uint = 0
	for operationsCounter < numberOfOperations {
		for it := uint(0); it < txRate; it++ {
			operationsCount, txError := singleTransaction(database, submitter, sampler)
			if txError != nil {
				Logger.Printf("error while committing a transaction: %s", txError)
			} else {
				operationsCounter += operationsCount
			}
		}
		select {
		case <-cancellation:
			return
		case <-ticker.C:
		}
	}
}

func singleTransaction(database Database, submitter TxSubmitter, sampler TransactionGenerator) (uint, error) {
	var operationsCount uint = 0
	database.BeginTransaction()
	defer database.EndTransaction()

	Logger.Printf("sampling data")
	data, sourceAccount := sampler(1, database)
	Logger.Printf("data sampled %+v", &data)
	if data == nil || sourceAccount == nil {
		Logger.Printf("unable to generate correct transaction, continuing...")
		return operationsCount, errors.New("unable to generate correct transaction")
	}
	operationsCount = uint(len(data.TX.Operations))
	Logger.Print("submitting tx")
	submitResult, sequenceUpdate, transactionResult := submitter.Submit(sourceAccount, data)
	if submitResult.Err != nil {
		Logger.Printf("tx submit rejected: %s", submitResult.Err)
		database = handleTransactionError(database, transactionResult)

		return operationsCount, errors.New("tx submit rejected")
	}
	Logger.Print("tx submitted")
	return operationsCount, nil

	Logger.Print("waiting for tx to externalize (seqnum increase)")
	newSequenceNum, seqError := sequenceUpdate()
	if seqError != nil {
		Logger.Print("error while checking if tx was externalized; downloading tx result")
		database = handleTransactionError(database, transactionResult)

		return operationsCount, errors.New("error while checking if tx was externalized")
	}
	sourceAccount.SeqNum = xdr.SequenceNumber(newSequenceNum.Sequence)

	if Debug {
		// txResult, txError := transactionResult()
		// if txError != nil {
		// 	Logger.Printf("error while downloading tx result: %+v", txError)
		// } else {
		// 	Logger.Printf("tx result: %+v", txResult)
		// }
		database = handleTransactionError(database, transactionResult)
	}

	Logger.Print("tx externalized, finishing")
	return operationsCount, nil
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

func failureDetector(postgresConnectionString string) {
	coreDb := NewDbSession(postgresConnectionString)
	// TODO move this magic number as a parameter
	iterator := NewTxResultIterator(FindFailedTransactions(coreDb, 700))
	noError := true
	for hasNext, error := iterator.Next(); hasNext; hasNext, error = iterator.Next() {
		noError = false
		if error != nil {
			Logger.Printf("error while iterating transactions: %s", error)
		}
		tx := iterator.Get()
		fmt.Println("------------------------")
		PrintTxErrors(*tx)
		fmt.Println()
		fmt.Println("------------------------")
	}
	if noError {
		fmt.Println("no tx error")
	}
}

func PrintTxErrors(result txsub.Result) {
	var xdrResult xdr.TransactionResult
	resultError := xdr.SafeUnmarshalBase64(result.ResultXDR, &xdrResult)
	if resultError != nil {

	}
	var envelope xdr.TransactionEnvelope
	envError := xdr.SafeUnmarshalBase64(result.EnvelopeXDR, &envelope)
	if envError != nil {

	}
	PrintErrors(xdrResult, envelope)
}

func PrintErrors(result xdr.TransactionResult, envelope xdr.TransactionEnvelope) {
	var val1, val2 string
	for ix, value := range *result.Result.Results {
		resultTr := value.Tr
		switch resultTr.Type {
		case xdr.OperationTypeCreateAccount: {
			if resultTr.CreateAccountResult.Code != xdr.CreateAccountResultCodeCreateAccountSuccess {
				createOp := envelope.Tx.Operations[ix].Body.CreateAccountOp
				val1 = "Create account\n" + toString(resultTr.CreateAccountResult)
				val2 = toString(createOp)
			}
		}
		case xdr.OperationTypePayment: {
			if resultTr.PaymentResult.Code != xdr.PaymentResultCodePaymentSuccess {
				paymentOp := envelope.Tx.Operations[ix].Body.PaymentOp
				val1 = "Payment\n" + toString(resultTr.PaymentResult)
				val2 = toString(paymentOp)
			}
		}
		}
		fmt.Println("-----")
		fmt.Println(val1)
		fmt.Println("###")
		fmt.Println(val2)
		fmt.Println("-----")
	}
}

func toString(value interface{}) string {
	b, _ := json.MarshalIndent(value, "", "  ")
	return string(b)
}

func PrintTxResult(result xdr.TransactionResultResult) {
	fmt.Printf("Code: %d", result.Code)
	fmt.Println()
	for _, value := range *result.Results {
		b, _ := json.MarshalIndent(value.Tr, "", "  ")
		fmt.Println("Result:")
		fmt.Println(string(b))
	}
}

func main() {
	// test()
	// return

	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
	stellarCoreURL := flag.String("core", "http://localhost:11626", "stellar-core http endpoint's url")
	flag.Parse()

	failureDetector(*postgresConnectionString)
	return

	var cancellation chan struct{} = make(chan struct{}, 2)
	defer func() {
		cancellation <- struct{}{}
		close(cancellation)
	}()

	setupSignalHandler(cancellation)

	const txRate, numberOfOperations, expectedNumberOfAccounts uint = 10, 1000000, 10000

	samplerLoop(*postgresConnectionString, *stellarCoreURL, cancellation, txRate, numberOfOperations, expectedNumberOfAccounts)
}
