package wallet

import (
	"database/sql"

	"github.com/stellar/go/support/errors"
)

// Scan implements sql.Scanner
func (kdf *KdfParams) Scan(src interface{}) error {
	// TODO
	return errors.New("TODO")
}

var _ sql.Scanner = &KdfParams{}
