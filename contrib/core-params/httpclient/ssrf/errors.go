package ssrf

import (
	"errors"
)

var (
	// ErrProhibitedNetwork is returned when trying to dial a destination whose
	// network type is not in our allow list
	ErrProhibitedNetwork = errors.New("prohibited network type")
	// ErrProhibitedPort is returned when trying to dial a destination on a port
	// number that's not in our allow list
	ErrProhibitedPort = errors.New("prohibited port number")
	// ErrProhibitedIP is returned when trying to dial a destionation whose IP
	// is on our deny list
	ErrProhibitedIP = errors.New("prohibited IP address")
	// ErrInvalidHostPort is returned when [netip.ParseAddrPort] is unable to
	// parse our destination into its host and port constituents
	ErrInvalidHostPort = errors.New("invalid host:port pair")
)
