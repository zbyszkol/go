package wallet

import (
	"context"

	"github.com/pkg/errors"
)

func (p *Protocol) FinishLogin(ctx context.Context) (*LockedWallet, error) {
	return nil, nil
}

// StartLogin retrieves the login parameters for the client provided username
// from the server.  These params, can be used to derive the WalletID capable of
// completing a login request.
func (p *Protocol) StartLogin(ctx context.Context) (*LoginParams, error) {
	username, err := p.Client.ReadUsername()
	if err != nil {
		return nil, errors.Wrap(err, "read username failed")
	}

	lp, err := p.Server.GetLoginParams(username)
	if err != nil {
		return nil, errors.Wrap(err, "server call failed")
	}

	return lp, nil
}

func (p *Protocol) Create(ctx context.Context) error {
	return nil
}
func (p *Protocol) Update(ctx context.Context) error {

	return nil
}

func (p *Protocol) Delete(ctx context.Context) error {

	return nil
}
