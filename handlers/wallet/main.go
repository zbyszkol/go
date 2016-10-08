// Package wallet implements an http handler that provides a REST accessible
// wallet server.
package wallet

import (
	"net/http"
	"time"

	"github.com/stellar/go/protocols/wallet"
)

// Driver represents a data source against which wallet queries can be run.  It
// represents the storage backend that this http handler uses to fulfill client
// request.
type Driver interface {
	// WalletByUsername finds a wallet by username
	WalletByUsername(username string) (*Record, error)

	// WalletByID finds a wallet by username while
	// verifying the provided id is correct.
	WalletByID(username string, id []byte) (*Record, error)
}

// Handler represents an http handler that exposes the server role of the wallet
// protocol.
type Handler struct {
	Driver
}

// Record represents a database row containing wallet information kept by a
// server in accordance to the wallet protocol.
type Record struct {
	LockVersion  int              `db:"lock_version"`
	Username     string           `db:"username"`
	WalletID     []byte           `db:"wallet_id"`
	Salt         string           `db:"salt"`
	KdfParams    wallet.KdfParams `db:"salt"`
	PublicKey    []byte           `db:"public_key"`
	MainData     []byte           `db:"main_data"`
	KeyChainData []byte           `db:"keychain_data"`
	CreatedAt    time.Time        `db:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at"`
	DeletedAt    time.Time        `db:"deleted_at"`
}

// Request represents an http request that can satisfy the client role of the
// wallet protocol.
type Request struct {
	HTTP *http.Request
}

// New creates a new instance of the wallet protocol http handler, using the
// provided driver as the storage to back the server.
func New(drv Driver) http.Handler {
	h := &Handler{drv}

	return h.Mux()
}
