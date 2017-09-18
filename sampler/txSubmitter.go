package sampler

import (
	"github.com/stellar/go/build"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
	"github.com/stellar/horizon/db2/core"
	"github.com/stellar/horizon/txsub"
	"golang.org/x/net/context"
	"net/http"
)

type TxSubmitter interface {
	Submit(txBuilder *build.TransactionBuilder, signers ...string) (txsub.SubmissionResult, func() *core.Transaction)
}

type SequenceNumberFetcher interface {
	FetchSequenceNumber(address keypair.KP) build.Sequence
}

type SequenceProvider struct {
	txsub.SequenceProvider
}

func (provider *SequenceProvider) FetchSequenceNumber(address keypair.KP) build.Sequence {
	addressString := address.Address()
	results, _ := provider.SequenceProvider.Get([]string{addressString})
	return build.Sequence{results[addressString]}
}

type txSubmitter struct {
	core      *core.Q
	submitter txsub.Submitter
	context   context.Context
}

func (submitter *txSubmitter) Submit(txBuilder *build.TransactionBuilder, signers ...string) (txsub.SubmissionResult, func() *core.Transaction) {
	txHash, _ := txBuilder.HashHex()
	envelope := txBuilder.Sign(signers...)
	envelopeString, _ := envelope.Base64()
	result := submitter.submitter.Submit(submitter.context, envelopeString)
	var resultFetcher func() *core.Transaction
	if result.Err != nil {
		resultFetcher = func() *core.Transaction {
			return nil
		}
	} else {
		resultFetcher = func() *core.Transaction {
			var result core.Transaction
			submitter.core.TransactionByHash(&result, txHash)
			return &result
		}
	}
	return result, resultFetcher
}

func NewTxSubmitter(h *http.Client, url string, psqlConnectionString string) TxSubmitter {
	submitter := txsub.NewDefaultSubmitter(h, url)
	txConfirmation := newTxConfirmation(psqlConnectionString)
	return &txSubmitter{core: txConfirmation, submitter: submitter}
}

func newTxConfirmation(psqlConnectionString string) *core.Q {
	session, err := db.Open("postgres", psqlConnectionString)

	if err != nil {
		panic(err)
	}

	session.DB.SetMaxIdleConns(4)
	session.DB.SetMaxOpenConns(12)
	return &core.Q{session}
}

// func (conf *txConfirmation) ResultByHash(ctx context.Context, hash string) txsub.Result {
// 	return conf.core.ResultByHash(ctx, hash)
// }

// func GetChangesSinceLedgerSeq(seqNumber xdr.SequenceNumber) ([][]xdr.OperationMeta, xdr.SequenceNumber, error) {
// }

// type Errors []error

// func (e Errors) Error() string {
// 	if len(e) == 1 {
// 		return e[0].Error()
// 	}
// 	msg := "multiple errors:"
// 	for _, err := range e {
// 		msg += "\n" + err.Error()
// 	}
// 	return msg
// }

// type TxConfirmation struct {
// 	stmt        sql.Stmt
// 	db          sql.DB
// 	queryString string
// }

// func (conf *TxConfirmation) Initialize() {
// 	dbinfo := ""
// 	conf.db, openError = sql.Open("postgres", dbinfo)
// 	if openError != nil {
// 		return openError
// 	}
// 	queryString := "SELECT txmeta FROM txhistory WHERE txid=$1 LIMIT 1"
// 	conf.stmt, stmtError = db.Prepare(queryString)
// 	if stmtError != nil {
// 		return stmtError
// 	}
// }

// func (conf *TxConfirmation) Dispose() error {
// 	stmtError := conf.stmt.Close()
// 	dbError := conf.db.Close()
// 	var errors Errors
// 	if stmtError != nil {
// 		errors = append(errors, stmtError)
// 	}
// 	if dbError != nil {
// 		errors = append(errors, dbError)
// 	}
// 	return errors
// }

// func GetTransactionResult(transactionHash string) ([]xdr.OperationMeta, error) {
// 	row := stmt.QueryRow(transactionHash)
// 	var txMetaBase64 string
// 	selectError := row.Scan(&txmeta)
// 	if selectError != nil {
// 		return nil, nil, selectError
// 	}
// 	txMeta := unmarshalTxMeta(txMetaBase64)
// 	return txMeta.Operations
// }

// // NOTE: this data type is stored inside of a txmeta structure
// // type LedgerEntryData struct {
// // 	Type      LedgerEntryType
// // 	Account   *AccountEntry
// // 	TrustLine *TrustLineEntry
// // 	Offer     *OfferEntry
// // 	Data      *DataEntry
// // }
