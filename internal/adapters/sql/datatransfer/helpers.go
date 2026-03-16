package datatransfer

// deref safely dereferences a pointer, returning the zero value of the type if
// the pointer is nil.
func deref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}

	return *ptr
}
