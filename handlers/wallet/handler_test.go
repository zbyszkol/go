package wallet

import (
	"net/http"
	"testing"

	"github.com/stellar/go/support/http/httptest"
)

func TestHandler(t *testing.T) {
	handler := &Handler{Driver: nil}
	server := httptest.NewServer(t, handler.Mux())
	defer server.Close()

	t.Run("/v2/kdf_params", func(t *testing.T) {
		server.GET("/v2/kdf_params").
			Expect().
			Status(http.StatusOK).
			JSON().Object().
			ContainsKey("algorithm").
			ValueEqual("algorithm", "scrypt").
			ContainsKey("n").
			ContainsKey("r").
			ContainsKey("p")
	})
}
