package ratelimiter

type PortFactory interface {
	RateLimiter() PortWrapped
}

//nolint:ireturn // standard port pattern: hiding implementation details
func New(factory PortFactory) PortWrapped { return factory.RateLimiter() }
