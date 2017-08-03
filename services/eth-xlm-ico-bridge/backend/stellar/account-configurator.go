package stellar

import (
	"time"

	"github.com/stellar/go/support/log"
)

func (ac *AccountConfigurator) Start() {
	ac.singleRun.Lock()
	defer ac.singleRun.Unlock()

	ac.log = ac.createLogger()
	ac.log.Info("StellarAccountConfigurator started")

	for {
		// Submit new transactions every second
		time.Sleep(time.Second)
	}
}

func (ac *AccountConfigurator) createLogger() *log.Entry {
	logger := log.New().WithField("service", "StellarAccountConfigurator")
	logger.Level = log.InfoLevel
	logger.Logger.Level = log.InfoLevel
	return logger
}

// ConfigureAccount configures a new account that participated in ICO.
// * First it creates a new account.
// * Once a trusline exists, it credits it with sent number of ETH.
func (s *AccountConfigurator) ConfigureAccount(publicKey string, ethAmount string) {
	// Check if account exists. If it is, skip creating it.
	s.createAccount(publicKey)
	// Wait for account to be created...
	s.sendEth(publicKey, ethAmount)
}

// CreateAccount adds a new transaction to create account with `publicKey` to the sending queue.
func (s *AccountConfigurator) createAccount(publicKey string) {
	//
}

// SendEth adds a new transaction to send ETH to account with `publicKey` to the sending queue.
func (s *AccountConfigurator) sendEth(publicKey string, amount string) {
	//
}
