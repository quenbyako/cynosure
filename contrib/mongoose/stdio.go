package goose

import (
	"context"
	"io"
	"os"
)

type ctxPipelineKey struct{}

type pipeline struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	isPipeline bool
}

func pipelineFromFiles(stdin, stdout, stderr *os.File) pipeline {
	return pipeline{
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		isPipeline: isPipeline(stdin),
	}
}

func defaultPipeline() pipeline {
	return pipelineFromFiles(os.Stdin, os.Stdout, os.Stderr)
}

func withPipelines(ctx context.Context, p pipeline) context.Context {
	return context.WithValue(ctx, ctxPipelineKey{}, p)
}

func pipelineFromContext(ctx context.Context) (pipeline, bool) {
	if p, ok := ctx.Value(ctxPipelineKey{}).(pipeline); ok {
		return p, true
	}

	return defaultPipeline(), false
}

func isPipeline(in *os.File) bool {
	stat := must(in.Stat())
	return (stat.Mode() & os.ModeCharDevice) == 0
}
