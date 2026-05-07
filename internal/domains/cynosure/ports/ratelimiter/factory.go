package ratelimiter

type PortFactory interface {
	RateLimiter() (PortWrapped, error)
}

func New(factory PortFactory) (PortWrapped, error) { return factory.RateLimiter() }
