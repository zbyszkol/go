package wallet

import (
	"net/http"
)

// Client represents an http client that conforms to the "client" role in the
// wallet protocol.  It can retrieve and unlock a wallet from a server.
type Client struct {
	HTTP *http.Client
}
