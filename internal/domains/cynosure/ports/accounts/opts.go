package accounts

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// WithIncludeDeleted sets the includeDeleted flag for the get account
// parameters.
func WithIncludeDeleted(p *getAccountParams) {
	p.includeDeleted = true
}

type (
	GetAccountOption interface{ applyGetAccount(p *getAccountParams) }

	getAccountFunc func(*getAccountParams)
)

var _ GetAccountOption = getAccountFunc(nil)

func (f getAccountFunc) applyGetAccount(p *getAccountParams) { f(p) }

// ========================================================================== //
//                           [PortRead.GetAccount]                            //
// ========================================================================== //

type getAccountRequiredParams struct {
	account ids.AccountID
}

type getAccountParams struct {
	getAccountRequiredParams
	includeDeleted bool
}

func GetAccountParams(
	account ids.AccountID, opts ...GetAccountOption,
) (getAccountParams, error) {
	params := defaultGetAccountParams(getAccountRequiredParams{
		account: account,
	})
	for _, opt := range opts {
		opt.applyGetAccount(&params)
	}

	if err := params.validate(); err != nil {
		return getAccountParams{}, err
	}

	return params, nil
}

func (s *getAccountParams) Account() ids.AccountID { return s.account }
func (s *getAccountParams) IncludeDeleted() bool   { return s.includeDeleted }
