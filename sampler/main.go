package sampler

import (
	"github.com/stellar/go/xdr"
	"math/rand"
)

func SamplerLoop() {
	var sampler TransactionGenerator = NewTransactionGenerator()
	var database Database = NewInMemoryDatabase()
	var submitter TxSubmitter
	var cancellation chan struct{} = make(chan struct{})
	defer close(cancellation)

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
			coreResult, _ := transactionResult()
			// if resultError != nil {
			// }
			database = ApplyChanges(&coreResult.ResultMeta, database)
		}
		sourceAccount.SeqNum = xdr.SequenceNumber(newSequenceNum.Sequence)

		database.EndTransaction()
	}
}
