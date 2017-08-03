package ethereum

import (
	"sync"

	"github.com/stellar/go/support/log"
)

type Listener struct {
	ReceivingAddress   string
	TransactionHandler TransactionHandler

	log       *log.Entry
	singleRun sync.Mutex
}

type TransactionHandler func(transaction Transaction)

type Transaction struct {
	From  string `json:"from"`
	Input string `json:"input"`
	Value string `json:"value"`
}
