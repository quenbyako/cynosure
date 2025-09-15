package onelog

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"time"
)

type Logger interface {
	Debug() Event
	Info() Event
	Warn() Event
	Error() Event
	Fatal() Event

	Err(err error) Event

	WithLevel(level slog.Level) Event
}

type logger struct {
	handler slog.Handler
}

var _ Logger = (*logger)(nil)

func Wrap(handler slog.Handler) Logger {
	return &logger{
		handler: handler,
	}
}

func (l *logger) Debug() Event { return l.WithLevel(slog.LevelDebug) }
func (l *logger) Info() Event  { return l.WithLevel(slog.LevelInfo) }
func (l *logger) Warn() Event  { return l.WithLevel(slog.LevelWarn) }
func (l *logger) Error() Event { return l.WithLevel(slog.LevelError) }
func (l *logger) Fatal() Event { return l.WithLevel(slog.LevelError - 4) }

func (l *logger) Err(err error) Event { return l.Error().AnErr("error", err) }

func (l *logger) WithLevel(level slog.Level) Event {
	if !l.handler.Enabled(context.Background(), level) {
		return &event{handler: nil}
	}

	return &event{handler: l.handler, record: slog.Record{
		Level: level,
	}}
}

type Event interface {
	Enabled() bool
	GetCtx() context.Context
	Send()
	Msg(msg string)
	MsgFunc(createMsg func() string)
	Msgf(format string, v ...any)

	AnErr(key string, err error) Event
	Any(key string, i any) Event
	Array(key string, arr LogArrayMarshaler) Event
	Bool(key string, b bool) Event
	Bools(key string, b []bool) Event
	Bytes(key string, val []byte) Event
	Caller(skip int) Event
	Ctx(ctx context.Context) Event
	Dict(key string, dict Event) Event
	Discard() Event
	Dur(key string, d time.Duration) Event
	Durs(key string, d []time.Duration) Event
	EmbedObject(obj LogObjectMarshaler) Event
	Err(err error) Event
	Errs(key string, errs []error) Event
	Fields(fields any) Event
	Float32(key string, f float32) Event
	Float64(key string, f float64) Event
	Floats32(key string, f []float32) Event
	Floats64(key string, f []float64) Event
	Func(f func(e Event)) Event

	Hex(key string, val []byte) Event
	IPAddr(key string, ip net.IP) Event
	IPPrefix(key string, pfx net.IPNet) Event
	Int(key string, i int) Event
	Int16(key string, i int16) Event
	Int32(key string, i int32) Event
	Int64(key string, i int64) Event
	Int8(key string, i int8) Event
	Interface(key string, i any) Event
	Ints(key string, i []int) Event
	Ints16(key string, i []int16) Event
	Ints32(key string, i []int32) Event
	Ints64(key string, i []int64) Event
	Ints8(key string, i []int8) Event
	MACAddr(key string, ha net.HardwareAddr) Event
	Object(key string, obj LogObjectMarshaler) Event
	RawCBOR(key string, b []byte) Event
	RawJSON(key string, b []byte) Event
	Stack() Event
	Str(key, val string) Event
	Stringer(key string, val fmt.Stringer) Event
	Stringers(key string, vals []fmt.Stringer) Event
	Strs(key string, vals []string) Event
	Time(key string, t time.Time) Event
	TimeDiff(key string, t time.Time, start time.Time) Event
	Times(key string, t []time.Time) Event
	Timestamp() Event
	Type(key string, val any) Event
	Uint(key string, i uint) Event
	Uint16(key string, i uint16) Event
	Uint32(key string, i uint32) Event
	Uint64(key string, i uint64) Event
	Uint8(key string, i uint8) Event
	Uints(key string, i []uint) Event
	Uints16(key string, i []uint16) Event
	Uints32(key string, i []uint32) Event
	Uints64(key string, i []uint64) Event
	Uints8(key string, i []uint8) Event
}

type Array interface {
	Bool(b bool) Array
	Bytes(val []byte) Array
	Dict(dict *Event) Array
	Dur(d time.Duration) Array
	Err(err error) Array
	Float32(f float32) Array
	Float64(f float64) Array
	Hex(val []byte) Array
	IPAddr(ip net.IP) Array
	IPPrefix(pfx net.IPNet) Array
	Int(i int) Array
	Int16(i int16) Array
	Int32(i int32) Array
	Int64(i int64) Array
	Int8(i int8) Array
	Interface(i interface{}) Array
	MACAddr(ha net.HardwareAddr) Array
	Object(obj LogObjectMarshaler) Array
	RawJSON(val []byte) Array
	Str(val string) Array
	Time(t time.Time) Array
	Uint(i uint) Array
	Uint16(i uint16) Array
	Uint32(i uint32) Array
	Uint64(i uint64) Array
	Uint8(i uint8) Array
}

type LogArrayMarshaler interface {
	MarshalZerologArray(a Array)
}

type LogObjectMarshaler interface {
	MarshalObjectInEvent(e Event)
}

type event struct {
	handler slog.Handler // if handler nil — means that event is NoOp

	ctx    context.Context
	record slog.Record
}

var _ Event = (*event)(nil)

func (e *event) Msg(msg string) {
	if e.handler == nil {
		return
	}

	e.record.Message = msg
	e.Send()
}
func (e *event) MsgFunc(createMsg func() string) {
	if e.handler == nil {
		return
	}

	e.record.Message = createMsg()
	e.Send()
}
func (e *event) Msgf(format string, v ...any) {
	if e.handler == nil {
		return
	}

	e.record.Message = fmt.Sprintf(format, v...)
	e.Send()
}

func (e *event) Send() {
	if e.handler == nil {
		return
	}

	ctx := e.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	e.handler.Handle(ctx, e.record)
}

func (e *event) AnErr(key string, err error) Event {
	if e.handler == nil {
		return e
	}

	errText := "<nil>"
	if err != nil {
		errText = err.Error()
	}
	e.record.AddAttrs(slog.String(key, errText))

	return e
}

// Any implements Event.
func (e *event) Any(key string, i any) Event {
	if e.handler == nil {
		return e
	}

	e.record.AddAttrs(slog.Any(key, i))
	return e
}

// Array implements Event.
func (e *event) Array(key string, arr LogArrayMarshaler) Event { panic("unimplemented") }

func (e *event) Bool(key string, b bool) Event {
	if e.handler == nil {
		return e
	}

	e.record.AddAttrs(slog.Bool(key, b))
	return e
}

func (e *event) Bools(key string, b []bool) Event {
	if e.handler == nil {
		return e
	}

	slog.Any(key, b)
	return e
}

func (e *event) Bytes(key string, val []byte) Event {
	if e.handler == nil {
		return e
	}

	slog.Any(key, val)
	return e
}

func (e *event) Caller(skip int) Event {
	if e.handler == nil {
		return e
	}

	e.record.PC, _, _, _ = runtime.Caller(skip)
	return e
}

func (e *event) Ctx(ctx context.Context) Event {
	if e.handler == nil {
		return e
	}

	e.ctx = ctx
	return e
}

func (e *event) Dict(key string, dict Event) Event               { panic("unimplemented") }
func (e *event) Discard() Event                                  { panic("unimplemented") }
func (e *event) Dur(key string, d time.Duration) Event           { panic("unimplemented") }
func (e *event) Durs(key string, d []time.Duration) Event        { panic("unimplemented") }
func (e *event) EmbedObject(obj LogObjectMarshaler) Event        { panic("unimplemented") }
func (e *event) Enabled() bool                                   { panic("unimplemented") }
func (e *event) Err(err error) Event                             { panic("unimplemented") }
func (e *event) Errs(key string, errs []error) Event             { panic("unimplemented") }
func (e *event) Fields(fields any) Event                         { panic("unimplemented") }
func (e *event) Float32(key string, f float32) Event             { panic("unimplemented") }
func (e *event) Float64(key string, f float64) Event             { panic("unimplemented") }
func (e *event) Floats32(key string, f []float32) Event          { panic("unimplemented") }
func (e *event) Floats64(key string, f []float64) Event          { panic("unimplemented") }
func (e *event) Func(f func(e Event)) Event                      { panic("unimplemented") }
func (e *event) GetCtx() context.Context                         { panic("unimplemented") }
func (e *event) Hex(key string, val []byte) Event                { panic("unimplemented") }
func (e *event) IPAddr(key string, ip net.IP) Event              { panic("unimplemented") }
func (e *event) IPPrefix(key string, pfx net.IPNet) Event        { panic("unimplemented") }
func (e *event) Int(key string, i int) Event                     { panic("unimplemented") }
func (e *event) Int16(key string, i int16) Event                 { panic("unimplemented") }
func (e *event) Int32(key string, i int32) Event                 { panic("unimplemented") }
func (e *event) Int64(key string, i int64) Event                 { panic("unimplemented") }
func (e *event) Int8(key string, i int8) Event                   { panic("unimplemented") }
func (e *event) Interface(key string, i any) Event               { panic("unimplemented") }
func (e *event) Ints(key string, i []int) Event                  { panic("unimplemented") }
func (e *event) Ints16(key string, i []int16) Event              { panic("unimplemented") }
func (e *event) Ints32(key string, i []int32) Event              { panic("unimplemented") }
func (e *event) Ints64(key string, i []int64) Event              { panic("unimplemented") }
func (e *event) Ints8(key string, i []int8) Event                { panic("unimplemented") }
func (e *event) MACAddr(key string, ha net.HardwareAddr) Event   { panic("unimplemented") }
func (e *event) Object(key string, obj LogObjectMarshaler) Event { panic("unimplemented") }
func (e *event) RawCBOR(key string, b []byte) Event              { panic("unimplemented") }
func (e *event) RawJSON(key string, b []byte) Event              { panic("unimplemented") }
func (e *event) Stack() Event                                    { panic("unimplemented") }
func (e *event) Str(key string, val string) Event {
	if e.handler == nil {
		return e
	}

	e.record.AddAttrs(slog.String(key, val))
	return e
}
func (e *event) Stringer(key string, val fmt.Stringer) Event             { panic("unimplemented") }
func (e *event) Stringers(key string, vals []fmt.Stringer) Event         { panic("unimplemented") }
func (e *event) Strs(key string, vals []string) Event                    { panic("unimplemented") }
func (e *event) Time(key string, t time.Time) Event                      { panic("unimplemented") }
func (e *event) TimeDiff(key string, t time.Time, start time.Time) Event { panic("unimplemented") }
func (e *event) Times(key string, t []time.Time) Event                   { panic("unimplemented") }
func (e *event) Timestamp() Event                                        { panic("unimplemented") }
func (e *event) Type(key string, val any) Event                          { panic("unimplemented") }
func (e *event) Uint(key string, i uint) Event                           { panic("unimplemented") }
func (e *event) Uint16(key string, i uint16) Event                       { panic("unimplemented") }
func (e *event) Uint32(key string, i uint32) Event                       { panic("unimplemented") }
func (e *event) Uint64(key string, i uint64) Event                       { panic("unimplemented") }
func (e *event) Uint8(key string, i uint8) Event                         { panic("unimplemented") }
func (e *event) Uints(key string, i []uint) Event                        { panic("unimplemented") }
func (e *event) Uints16(key string, i []uint16) Event                    { panic("unimplemented") }
func (e *event) Uints32(key string, i []uint32) Event                    { panic("unimplemented") }
func (e *event) Uints64(key string, i []uint64) Event                    { panic("unimplemented") }
func (e *event) Uints8(key string, i []uint8) Event                      { panic("unimplemented") }
