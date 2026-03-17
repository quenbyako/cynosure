package oauthhandler

type Factory interface {
	OAuthHandler() PortWrapped
}

//nolint:ireturn // standard port pattern: hiding implementation details
func New(factory Factory) PortWrapped { return factory.OAuthHandler() }
