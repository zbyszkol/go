package stellar

import (
	"sync"

	"github.com/stellar/go/support/log"
)

// AccountConfigurator is responsible for configuring new Stellar accounts that
// participate in ICO.
type AccountConfigurator struct {
	singleRun sync.Mutex
	log       *log.Entry
}
