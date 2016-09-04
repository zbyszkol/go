package horizon

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_ResultCodes(t *testing.T) {
	var herr Error

	// happy path: transaction_failed with the appropriate extra fields
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]json.RawMessage)
	herr.Problem.Extras["result_codes"] = json.RawMessage(`{
    "transaction": "tx_failed",
    "operations": ["op_underfunded"]
  }`)

	trc, err := herr.ResultCodes()
	if assert.NoError(t, err) {
		assert.Equal(t, "tx_failed", trc.TransactionCode)

		if assert.Len(t, trc.OperationCodes, 1) {
			assert.Equal(t, "op_underfunded", trc.OperationCodes[0])
		}
	}

	// sad path: !transaction_failed
	herr.Problem.Type = "transaction_success"
	herr.Problem.Extras = make(map[string]json.RawMessage)
	_, err = herr.ResultCodes()
	assert.Equal(t, ErrTransactionNotFailed, err)

	// sad path: missing result_codes extra
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]json.RawMessage)
	_, err = herr.ResultCodes()
	assert.Equal(t, ErrResultCodesNotPopulated, err)

	// sad path: unparseable result_codes extra
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]json.RawMessage)
	herr.Problem.Extras["result_codes"] = json.RawMessage(`kaboom`)
	_, err = herr.ResultCodes()
	assert.Error(t, err)
}
