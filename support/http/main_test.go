package http

import (
	"errors"
	stdhttp "net/http"
	"testing"

	"net/http/httptest"

	"encoding/json"

	"github.com/stretchr/testify/assert"
)

func TestRun_setup(t *testing.T) {

	// test that using no handler panics
	assert.Panics(t, func() {
		setup(Config{
			Handler: nil,
		})
	})

	// test defaults
	srv := setup(Config{
		Handler: stdhttp.NotFoundHandler(),
	})

	assert.Equal(t, DefaultShutdownGracePeriod, srv.Timeout)
	assert.Equal(t, DefaultListenAddr, srv.Server.Addr)
}

func TestWriteJSON(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		response := struct {
			MemoType string `json:"memo_type"`
		}{"text"}

		rec := httptest.NewRecorder()
		WriteJSON(rec, response, 201)

		assert.Equal(t, 201, rec.Code)
		assert.Contains(t, rec.Body.String(), "memo_type")
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	})

	t.Run("default status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteJSON(rec, "hello", 0)
		assert.Equal(t, 200, rec.Code)
	})

	t.Run("response fails to encode ", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteJSON(rec, noJSON(1), 0)
		assert.Equal(t, 500, rec.Code)
		assert.Contains(t, rec.Body.String(), "internal error occurred")
	})
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, errors.New("boom"))
	assert.Contains(t, rec.Body.String(), "internal error occurred")
	assert.NotContains(t, rec.Body.String(), "boom")
	// TODO: test that the log line is emitted correctly
}

type noJSON int

func (j noJSON) MarshalJSON() ([]byte, error) {
	return nil, errors.New("no json for you")
}

var _ json.Marshaler = noJSON(0)
