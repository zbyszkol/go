package wallet

import (
	"net/http"

	"github.com/stellar/go/protocols/wallet"
)

type remote struct {
	HTTP *http.Client
}

var _ wallet.Server = &remote{}
