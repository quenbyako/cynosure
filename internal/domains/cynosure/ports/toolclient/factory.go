package toolclient

// PortFactory creates [Port] instances.
type PortFactory interface {
	ToolClient() PortWrapped
}

//nolint:ireturn // standard port pattern: hiding implementation details
func New(f PortFactory) PortWrapped {
	return f.ToolClient()
}
