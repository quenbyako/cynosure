package identitymanager

type PortFactory interface {
	IdentityManager() PortWrapped
}

func New(factory PortFactory) PortWrapped { return factory.IdentityManager() }
