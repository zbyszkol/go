package wallet

import (
	"encoding/json"

	"github.com/asaskevich/govalidator"
	"github.com/pkg/errors"
	"github.com/stellar/go/protocols/wallet"
)

// ReadUsername implements wallet.Client
func (req *Request) ReadUsername() (string, error) {
	var body struct {
		Username string `json:"username" valid:"required,length(2|255)"`
	}

	dec := json.NewDecoder(req.HTTP.Body)
	err := dec.Decode(&body)
	if err != nil {
		return "", errors.Wrap(err, "decode body failed")
	}

	_, err = govalidator.ValidateStruct(body)
	if err != nil {
		return "", errors.Wrap(err, "body validation failed")
	}

	return body.Username, nil
}

var _ wallet.Client = &Request{}
