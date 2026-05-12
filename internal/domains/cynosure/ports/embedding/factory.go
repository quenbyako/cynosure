package embedding

type PortFactory interface {
	Embedding() PortWrapped
}

func New(factory PortFactory) PortWrapped { return factory.Embedding() }
