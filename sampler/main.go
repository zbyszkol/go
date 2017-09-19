package sampler

import (
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
		size := Size(rand.Int(100) + 1)
		data, sourceAccount := sampler(size, database)
		submitResult, seqenceUpdate, transactionResult := submitter.Submit(sourceAccount, data)
		if submitResult.Err != nil {
			// TODO scream and run away, don't forget to take dog with you
			database.RejectTransaction()
			continue
		}
		newSequenceNum, seqError := seqenceUpdate()
		sourceAccount.SeqNum = newSequenceNum
		database.EndTransaction()
	}
}
