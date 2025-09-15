package goose

func must[T any](t T, err error) T { //nolint:ireturn // false positive on generics
	if err != nil {
		panic(err)
	}

	return t
}

func optional[T any](t *T, def T) T {
	if t == nil {
		return def
	}

	return *t
}
