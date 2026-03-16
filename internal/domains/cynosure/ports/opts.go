package ports

import (
	"go.opentelemetry.io/otel/trace"
)

// Unlike common option that expects TracerProvider, this option expects
// initialized metrics provider, that will be converted into Observable.
//
// Applies to:
//
//   - [WrapThreadStorage]
func WithTrace(trace trace.Tracer) traceWrapper {
	return traceWrapper{trace: trace}
}

type (
	WrapThreadStorageOption interface{ applyWrapThreadStorage(*threadStorageWrapped) }

	traceWrapper struct{ trace trace.Tracer }
)

//nolint:exhaustruct // interface check
var (
	_ WrapThreadStorageOption = traceWrapper{}
)

func (t traceWrapper) applyWrapThreadStorage(p *threadStorageWrapped) { p.trace = t.trace }
