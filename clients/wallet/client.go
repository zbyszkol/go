package wallet

import (
	"github.com/stellar/go/protocols/wallet"
)

var _ wallet.Client = &Client{}
