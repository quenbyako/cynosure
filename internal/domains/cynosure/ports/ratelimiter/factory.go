package ratelimiter

type PortFactory interface {
	RateLimiter() PortWrapped
}

func New(factory PortFactory) PortWrapped { return factory.RateLimiter() }
