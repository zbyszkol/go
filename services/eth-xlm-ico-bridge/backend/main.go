package backend

import (
	"github.com/stellar/go/services/eth-xlm-ico-bridge/backend/ethereum"
	"github.com/stellar/go/services/eth-xlm-ico-bridge/backend/stellar"
)

type PersistentStorage interface {
	//
}

type Server struct {
	EthereumListener           ethereum.Listener
	StellarAccountConfigurator stellar.AccountConfigurator
}
