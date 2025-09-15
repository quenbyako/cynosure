package goose

import "context"

type Version struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

type ctxVersionKey struct{}

func WithVersion(ctx context.Context, v Version) context.Context {
	return context.WithValue(ctx, ctxVersionKey{}, v)
}

func VersionFromContext(ctx context.Context) (Version, bool) {
	v, ok := ctx.Value(ctxVersionKey{}).(Version)
	return v, ok
}
