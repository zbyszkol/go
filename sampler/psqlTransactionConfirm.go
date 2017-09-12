package sampler

import (
	"fmt"
	"github.com/stellar/go/build"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
	"github.com/stellar/horizon/txsub"
	"golang.org/x/net/context"
	"time"
)

type TxSubmitter interface {
	Submit(envelope build.TransactionEnvelopeBuilder) (result txsub.SubmissionResult, confirm func() txsub.Result)
}

type txSubmitter struct {
	// stmt        sql.Stmt
	// db          sql.DB
	// queryString string
	core      *core.Q
	submitter txsub.Submitter
	context   context.Context
}

func (submitter *txSubmitter) Submit(envelope build.TransactionEnvelopeBuilder) (result txsub.SubmissionResult, confirm func() txsub.Result) {

}

func NewTxSubmitter(h *http.Client, url string, psqlConnectionString string) TxSubmitter {
	submitter := txsub.NewDefaultSubmitter(h, url)
	txConfirmation := newTxConfirmation(psqlConnectionString)
	return &txSubmitter{core: txConfirmation, submitter: submitter}
}

// func (sub *submitter) Submit(envelope string) (result txsub.SubmissionResult) {
// 	submitter.Submit(context.Context.TODO(), envelope)
// }

func newTxConfirmation(psqlConnectionString string) *core.Q {
	session, err := db.Open("postgres", psqlConnectionString)

	if err != nil {
		panic(err)
	}

	session.DB.SetMaxIdleConns(4)
	session.DB.SetMaxOpenConns(12)
	return &session
}

// func (conf *txConfirmation) ResultByHash(ctx context.Context, hash string) txsub.Result {
// 	return conf.core.ResultByHash(ctx, hash)
// }

func UnmarshalTxMeta(data string) (result xdr.TransactionMeta) {
	xdr.SafeUnmarshalBase64(data, &result)
}

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
