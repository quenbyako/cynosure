package chatmodel

type PortFactory interface {
	ChatModel() PortWrapped
}

func New(factory PortFactory) PortWrapped { return factory.ChatModel() }
