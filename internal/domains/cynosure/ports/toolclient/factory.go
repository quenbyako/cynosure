package toolclient

// PortFactory creates [Port] instances.
type PortFactory interface {
	ToolClient() PortWrapped
}

func New(f PortFactory) PortWrapped {
	return f.ToolClient()
}
