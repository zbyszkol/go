package sampler

import (
	"errors"
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
	Submit(sourceAccount *AccountEntry, txBuilder *build.TransactionBuilder) (txsub.SubmissionResult, func() (*build.Sequence, error), func() (*core.Transaction, error))
}

type SequenceNumberFetcher interface {
	FetchSequenceNumber(address keypair.KP) (build.Sequence, error)
}

type AccountFetcher interface {
	FetchAccount(address keypair.KP) (*core.Account, error)
}

type SequenceProvider struct {
	txsub.SequenceProvider
}

func (provider *SequenceProvider) FetchSequenceNumber(address keypair.KP) (build.Sequence, error) {
	var result build.Sequence
	addressString := address.Address()
	sequences, error := provider.SequenceProvider.Get([]string{addressString})
	if error != nil {
		return result, error
	}
	var ok bool
	result.Sequence, ok = sequences[addressString]
	if !ok {
		return result, errors.New("missing sequence number")
	}
	Logger.Printf("fetched sequence number (%+v) for account %s", result, address)
	return result, nil
}

func (submitter *txSubmitter) FetchSequenceNumber(address keypair.KP) (build.Sequence, error) {
	return submitter.sequenceProvider.FetchSequenceNumber(address)
}

type txSubmitter struct {
	core             *core.Q
	submitter        txsub.Submitter
	context          context.Context
	sequenceProvider SequenceNumberFetcher
}

func (submitter *txSubmitter) Submit(sourceAccount *AccountEntry, txBuilder *build.TransactionBuilder) (txsub.SubmissionResult, func() (*build.Sequence, error), func() (*core.Transaction, error)) {
	Logger.Printf("transaction to submit: %+v", txBuilder)
	txHash, _ := txBuilder.HashHex()
	envelope := txBuilder.Sign(sourceAccount.Keypair.GetSeed().Seed())
	envelopeString, baseErr := envelope.Base64()
	if baseErr != nil {
		Logger.Printf("error while serializing an envelope: %s", baseErr)
	}
	Logger.Printf("submitting tx: %s", envelopeString)
	result := submitter.submitter.Submit(submitter.context, envelopeString)
	var sequenceFetcher func() (*build.Sequence, error)
	var resultFetcher func() (*core.Transaction, error)
	if result.Err != nil {
		sequenceFetcher = func() (*build.Sequence, error) {
			return nil, nil
		}
		resultFetcher = func() (*core.Transaction, error) {
			return nil, nil
		}
	} else {
		sequenceFetcher = waitForNewSequenceNumber(submitter.sequenceProvider, sourceAccount)
		resultFetcher = waitForTransactionResult(submitter.core, txHash)
	}
	return result, sequenceFetcher, resultFetcher
}

func (submitter *txSubmitter) FetchAccount(address keypair.KP) (*core.Account, error) {
	var account core.Account
	error := submitter.core.AccountByAddress(&account, address.Address())
	if error != nil {
		return nil, errors.New("error while fetching an account")
	}
	return &account, nil
}

func waitForTransactionResultOnce(coreQ *core.Q, txHash string) func() (*core.Transaction, error) {
	return func() (*core.Transaction, error) {
		var result core.Transaction
		error := coreQ.TransactionByHash(&result, txHash)
		if error != nil && !coreQ.NoRows(error) {
			return nil, error
		}
		return &result, nil
	}
}

func waitForTransactionResult(coreQ *core.Q, txHash string) func() (*core.Transaction, error) {
	return func() (*core.Transaction, error) {
		var result core.Transaction
		for {
			error := coreQ.TransactionByHash(&result, txHash)
			if error != nil {
				if !coreQ.NoRows(error) {
					return nil, error
				}
			} else {
				break
			}
		}
		return &result, nil
	}
}

func waitForNewSequenceNumberOnce(sequenceProvider SequenceNumberFetcher, account *AccountEntry) func() (*build.Sequence, error) {
	accountSeqNum := account.SeqNum
	return func() (*build.Sequence, error) {
		sequenceNumValue, fetchError := sequenceProvider.FetchSequenceNumber(account.Keypair.GetSeed())
		if fetchError != nil {
			return nil, errors.New("error while fetching a sequence number")
		}
		sequence := xdr.SequenceNumber(sequenceNumValue.Sequence)
		if sequence < accountSeqNum {
			return nil, errors.New("acount's sequence number is in the future")
		}
		if sequence != accountSeqNum {
			return &build.Sequence{uint64(sequence)}, nil
		}
		return nil, nil
	}
}

func waitForNewSequenceNumber(sequenceProvider SequenceNumberFetcher, account *AccountEntry) func() (*build.Sequence, error) {
	accountSeqNum := account.SeqNum
	return func() (*build.Sequence, error) {
		sequence := accountSeqNum
		for sequence == accountSeqNum {
			if sequence < accountSeqNum {
				return nil, errors.New("acount's sequence number is in the future")
			}
			sequenceNumValue, fetchError := sequenceProvider.FetchSequenceNumber(account.Keypair.GetSeed())
			if fetchError != nil {
				return nil, errors.New("error while fetching a sequence number")
			}
			sequence = xdr.SequenceNumber(sequenceNumValue.Sequence)
		}
		return &build.Sequence{uint64(sequence)}, nil
	}
}

func NewTxSubmitter(h *http.Client, url, psqlConnectionString string) (TxSubmitter, AccountFetcher, SequenceNumberFetcher) {
	submitter := txsub.NewDefaultSubmitter(h, url)
	coreDb := NewDbSession(psqlConnectionString)
	seqProvider := &SequenceProvider{coreDb.SequenceProvider()}
	result := &txSubmitter{core: coreDb, submitter: submitter, sequenceProvider: seqProvider}
	return result, result, result
}

func NewDbSession(psqlConnectionString string) *core.Q {
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
