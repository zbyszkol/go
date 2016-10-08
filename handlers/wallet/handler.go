package wallet

import (
	"net/http"

	"github.com/stellar/go/protocols/wallet"
	strhttp "github.com/stellar/go/support/http"
	"goji.io"
	"goji.io/pat"
)

// GetLoginParams implements wallet.Server
func (h *Handler) GetLoginParams(
	username string,
) (*wallet.LoginParams, error) {
	return nil, nil
}

// GetByWalletID implements wallet.Server
func (h *Handler) GetByWalletID(
	username string,
	walletid string,
) (*wallet.Wallet, error) {

	return nil, nil
}

// Mux returns an http.Handler capable of serving all the endpoints necessary
// for the server role of the wallet protocol.
func (h *Handler) Mux() http.Handler {
	mux := goji.NewMux()

	mux.HandleFunc(pat.Get("/v2/kdf_params"), h.kdfParams)

	return mux
}

func (h *Handler) kdfParams(w http.ResponseWriter, r *http.Request) {
	dkdf := wallet.DefaultKdfParams
	response := struct {
		Algo string `json:"algorithm"`
		Bits int    `json:"bits"`
		N    int    `json:"n"`
		R    int    `json:"r"`
		P    int    `json:"p"`
	}{
		Algo: "scrypt",
		Bits: dkdf.Bits,
		N:    dkdf.N,
		R:    dkdf.R,
		P:    dkdf.P,
	}

	strhttp.WriteJSON(w, response, http.StatusOK)
}

var _ wallet.Server = &Handler{}
