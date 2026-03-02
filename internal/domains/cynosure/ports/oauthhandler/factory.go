package oauthhandler

type Factory interface {
	OAuthHandler() PortWrapped
}

func New(factory Factory) PortWrapped { return factory.OAuthHandler() }
