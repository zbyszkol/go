package failureDetector

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	. "github.com/stellar/go/sampler"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
	"github.com/stellar/horizon/db2/core"
	"github.com/stellar/horizon/txsub"
)

type TransactionsIterator interface {
	Next() (bool, error)
	Get() *core.Transaction
}

type TxResultIterator interface {
	Next() (bool, error)
	Get() *txsub.Result
}

type txResultIterator struct {
	iterator TransactionsIterator
}

func NewTxResultIterator(iterator TransactionsIterator) TxResultIterator {
	return &txResultIterator{iterator}
}

func (impl *txResultIterator) Next() (bool, error) {
	return impl.iterator.Next()
}

func (impl *txResultIterator) Get() *txsub.Result {
	result := impl.iterator.Get()
	transformed := txResultFromCore(*result)
	return &transformed
}

type transactionsIteratorImpl struct {
	ledgerNumber uint64
	toLedger     uint64
	inLedgerIx   int
	txs          []core.Transaction
	core         *core.Q
}

type filteredTransactionsIteratorImpl struct {
	iterator TransactionsIterator
	filter   func(*core.Transaction) bool
}

func NewFilteredTransactionsIterator(filter func(*core.Transaction) bool, iterator TransactionsIterator) TransactionsIterator {
	return &filteredTransactionsIteratorImpl{iterator: iterator, filter: filter}
}

func (impl *filteredTransactionsIteratorImpl) Next() (bool, error) {
	Logger.Print("called filtered Next()")
	for hasNext, error := impl.iterator.Next(); hasNext; hasNext, error = impl.iterator.Next() {
		Logger.Println("searching to filter...")
		if !hasNext {
			Logger.Println("false")
			return hasNext, error
		}
		if impl.filter(impl.iterator.Get()) {
			Logger.Println("filtered")
			return true, nil
		}
	}
	Logger.Println("nothing left")
	return false, nil
}

func (impl *filteredTransactionsIteratorImpl) Get() *core.Transaction {
	return impl.iterator.Get()
}

func Filter(filterFunc func(*core.Transaction) bool, iterator TransactionsIterator) TransactionsIterator {
	return NewFilteredTransactionsIterator(filterFunc, iterator)
}

func NewTransactionsIterator(coreQ *core.Q, fromLedger, toLedger uint64) TransactionsIterator {
	return &transactionsIteratorImpl{ledgerNumber: fromLedger - 1, toLedger: toLedger, inLedgerIx: 0, txs: []core.Transaction{}, core: coreQ}
}

func (impl *transactionsIteratorImpl) Next() (bool, error) {
	if impl.ledgerNumber > impl.toLedger {
		return false, nil
	}
	impl.inLedgerIx++
	if len(impl.txs) > 0 && impl.inLedgerIx < len(impl.txs) {
		return true, nil
	}
	impl.inLedgerIx = 0
	impl.txs = []core.Transaction{}
	for impl.ledgerNumber++; impl.ledgerNumber <= impl.toLedger; impl.ledgerNumber++ {
		Logger.Printf("ledger number: %d\n", impl.ledgerNumber)
		error := impl.core.TransactionsByLedger(&impl.txs, int32(impl.ledgerNumber))
		if error != nil {
			// impl.ledgerNumber--
			Logger.Println("error while downloading transactions")
			return false, error
		}
		if len(impl.txs) > 0 {
			return true, nil
		}
	}
	Logger.Printf("ledger number after loop: %d\n", impl.ledgerNumber)
	return false, nil
}

func (impl *transactionsIteratorImpl) Get() *core.Transaction {
	return &impl.txs[impl.inLedgerIx]
}

func FindFailedTransactions(coreQ *core.Q, toLedgerNum uint64) TransactionsIterator {
	iterator := NewTransactionsIterator(coreQ, 1, toLedgerNum)
	filter := func(tx *core.Transaction) bool {
		return !tx.IsSuccessful()
	}
	return NewFilteredTransactionsIterator(filter, iterator)
}

func GetAllTransactions(core *core.Q, toLedgerNum uint64) TransactionsIterator {
	return NewTransactionsIterator(core, 1, toLedgerNum)
}

func txResultFromCore(tx core.Transaction) txsub.Result {
	// re-encode result to base64
	var raw bytes.Buffer
	_, err := xdr.Marshal(&raw, tx.Result.Result)

	if err != nil {
		return txsub.Result{Err: err}
	}

	trx := base64.StdEncoding.EncodeToString(raw.Bytes())

	// if result is success, send a normal resposne
	if tx.Result.Result.Result.Code == xdr.TransactionResultCodeTxSuccess {
		return txsub.Result{
			Hash:           tx.TransactionHash,
			LedgerSequence: tx.LedgerSequence,
			EnvelopeXDR:    tx.EnvelopeXDR(),
			ResultXDR:      trx,
			ResultMetaXDR:  tx.ResultMetaXDR(),
		}
	}

	// if failed, produce a FailedTransactionError
	return txsub.Result{
		Err: &txsub.FailedTransactionError{
			ResultXDR: trx,
		},
		Hash:           tx.TransactionHash,
		LedgerSequence: tx.LedgerSequence,
		EnvelopeXDR:    tx.EnvelopeXDR(),
		ResultXDR:      trx,
		ResultMetaXDR:  tx.ResultMetaXDR(),
	}
}

func TxResultXdrToObject(xdrValue string) (*xdr.TransactionResult, error) {
	var result xdr.TransactionResult
	error := xdr.SafeUnmarshalBase64(xdrValue, &result)
	if error != nil {
		return nil, error
	}
	return &result, nil
}

func PrintTxErrorsFromResult(result txsub.Result) {
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

func PrintFailuresFromCoreTx(tx *core.Transaction) {
	fmt.Printf("Ledger Sequence: %d", tx.LedgerSequence)
	fmt.Println()
	fmt.Printf("Index: %d", tx.Index)
	fmt.Println()
	PrintErrors(tx.Result.Result, tx.Envelope)
}

func AccountIdToString(id xdr.AccountId) string {
	return strkey.MustEncode(strkey.VersionByteAccountID, (*id.Ed25519)[:])
}

func PrintErrors(result xdr.TransactionResult, envelope xdr.TransactionEnvelope) {
	var val1, val2 string
	for ix, value := range *result.Result.Results {
		resultTr := value.Tr
		switch resultTr.Type {
		case xdr.OperationTypeCreateAccount:
			{
				if resultTr.CreateAccountResult.Code != xdr.CreateAccountResultCodeCreateAccountSuccess {
					createOp := envelope.Tx.Operations[ix].Body.CreateAccountOp
					val1 = "Create account:\n" + toString(resultTr.CreateAccountResult)
					val2 = toString(createOp)
					val2 += "\nSource Account: " + AccountIdToString(envelope.Tx.SourceAccount)
					val2 += "\nNew Account: " + AccountIdToString(createOp.Destination)
				}
			}
		case xdr.OperationTypePayment:
			{
				if resultTr.PaymentResult.Code != xdr.PaymentResultCodePaymentSuccess {
					paymentOp := envelope.Tx.Operations[ix].Body.PaymentOp
					val1 = "Payment:\n" + toString(resultTr.PaymentResult)
					val2 = toString(paymentOp)
					val2 += "\nSource Account: " + AccountIdToString(envelope.Tx.SourceAccount)
					val2 += "\nDestination Account: " + AccountIdToString(paymentOp.Destination)
				}
			}
		}
		if len(val1) == 0 {
			return
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
