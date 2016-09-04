package horizon

import (
	"encoding/json"

	"github.com/stellar/go/support/errors"
)

func (herr *Error) Error() string {
	// TODO: use the attached problem to provide a better error message
	return "Horizon error"
}

// ResultCodes extracts a result code summary from the error, if possible.
// NOTE: this method will panic if the input that was received from horizon is
// invalid.
func (herr *Error) ResultCodes() (*TransactionResultCodes, error) {
	if herr.Problem.Type != "transaction_failed" {
		return nil, ErrTransactionNotFailed
	}

	raw, ok := herr.Problem.Extras["result_codes"]
	if !ok {
		return nil, ErrResultCodesNotPopulated
	}

	var result TransactionResultCodes
	err := json.Unmarshal(raw, &result)
	if err != nil {
		return nil, errors.Wrap(err, "json decode failed")
	}

	return &result, nil
}
