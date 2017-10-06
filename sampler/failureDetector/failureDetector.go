package failureDetector

import (
	"bytes"
	"encoding/base64"
	. "github.com/stellar/go/sampler"
	"github.com/stellar/go/xdr"
	"github.com/stellar/horizon/db2/core"
	"github.com/stellar/horizon/txsub"
)

// func main() {
// 	postgresConnectionString := flag.String("pg", "dbname=core host=localhost user=stellar password=__PGPASS__", "PostgreSQL connection string")
// 	coreDb := NewDbSession(*postgresConnectionString)
// 	iterator := FindFailedTransactions(coreDb, 9000)
// 	for hasNext, error := iterator.Next(); hasNext; hasNext, error = iterator.Next() {
// 		if error != nil {
// 			Logger.Printf("error while iterating transactions: %s", error)
// 		}
// 		tx := iterator.Get()
// 		fmt.Println("------------------------")
// 		fmt.Printf("%+v", tx)
// 		fmt.Println("------------------------")
// 	}
// }

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
	ledgerNumber uint32
	toLedger     uint32
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
		Logger.Printf("searching to filter...")
		if !hasNext {
			Logger.Printf("false")
			return hasNext, error
		}
		if impl.filter(impl.iterator.Get()) {
			Logger.Printf("filtered")
			return true, nil
		}
	}
	Logger.Printf("nothing left")
	return false, nil
}

func (impl *filteredTransactionsIteratorImpl) Get() *core.Transaction {
	return impl.iterator.Get()
}

func Filter(filterFunc func(*core.Transaction) bool, iterator TransactionsIterator) TransactionsIterator {
	return NewFilteredTransactionsIterator(filterFunc, iterator)
}

func NewTransactionsIterator(coreQ *core.Q, fromLedger, toLedger uint32) TransactionsIterator {
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
	impl.txs = nil
	for impl.ledgerNumber++; impl.ledgerNumber <= impl.toLedger; impl.ledgerNumber++ {
		Logger.Printf("ledger number: %d", impl.ledgerNumber)
		error := impl.core.TransactionsByLedger(&impl.txs, int32(impl.ledgerNumber))
		if error != nil {
			// impl.ledgerNumber--
			Logger.Printf("error while downloading transactions")
			return false, error
		}
		if len(impl.txs) > 0 {
			Logger.Printf("has next")
			return true, nil
		}
	}
	Logger.Printf("ledger number after loop: %d", impl.ledgerNumber)
	return false, nil
}

func (impl *transactionsIteratorImpl) Get() *core.Transaction {
	return &impl.txs[impl.inLedgerIx]
}

func FindFailedTransactions(coreQ *core.Q, toLedgerNum uint32) TransactionsIterator {
	iterator := NewTransactionsIterator(coreQ, 1, toLedgerNum)
	filter := func(tx *core.Transaction) bool {
		return !tx.IsSuccessful()
	}
	return NewFilteredTransactionsIterator(filter, iterator)
}

func GetAllTransactions(core *core.Q, toLedgerNum uint32) TransactionsIterator {
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
