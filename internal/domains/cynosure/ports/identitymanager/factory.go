package identitymanager

type PortFactory interface {
	IdentityManager() PortWrapped
}

//nolint:ireturn // hiding implementation details
func New(factory PortFactory) PortWrapped { return factory.IdentityManager() }
