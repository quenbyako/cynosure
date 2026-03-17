package chatmodel

type PortFactory interface {
	ChatModel() PortWrapped
}

//nolint:ireturn // it's a factory function
func New(factory PortFactory) PortWrapped { return factory.ChatModel() }
