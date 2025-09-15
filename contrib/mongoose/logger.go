package goose

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const useZerologFormatter = true

func defaultLogger(w io.Writer, level slog.Level) slog.Handler {
	opts := slog.HandlerOptions{
		Level: level,
		ReplaceAttr: multiReplacer(map[string]replacer{
			"source": sourceReplacer,
			"level":  levelReplacer,
		}),
	}

	handler := slog.NewJSONHandler(w, &opts)

	return handler
}

const (
	LevelTrace = slog.LevelDebug - 4
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
	LevelFatal = slog.LevelError + 4
	LevelPanic = slog.LevelError + 8
)

type replacer = func(groups []string, a slog.Attr) slog.Attr

func multiReplacer(replacers map[string]replacer) replacer {
	return func(groups []string, a slog.Attr) slog.Attr {
		if replacer, ok := replacers[a.Key]; ok {
			return replacer(groups, a)
		}

		return a
	}
}

func sourceReplacer(groups []string, a slog.Attr) slog.Attr {
	if a.Key != "source" || len(groups) > 0 {
		return a
	}

	if source, ok := a.Value.Any().(*slog.Source); ok {
		return slog.String("source", fmt.Sprintf("%s:%d", source.File, source.Line))
	}

	return a

}

func levelReplacer(groups []string, a slog.Attr) slog.Attr {
	if a.Key != "level" || len(groups) > 0 {
		return a
	}

	return slog.String("level", replaceLevel(a.Value.Any().(slog.Level)))
}

func replaceLevel(l slog.Level) string {
	str := func(base string, val slog.Level) string {
		if val == 0 {
			return base
		}

		return fmt.Sprintf("%s%+d", base, val)
	}

	switch {
	case l <= LevelTrace:
		return str("TRACE", l-LevelTrace)
	case l <= LevelDebug:
		return str("DEBUG", l-LevelDebug)
	case l <= LevelInfo:
		return str("INFO", l-LevelInfo)
	case l <= LevelWarn:
		return str("WARN", l-LevelWarn)
	case l <= LevelError:
		return str("ERROR", l-LevelError)
	case l <= LevelFatal:
		return str("FATAL", l-LevelFatal)
	default:
		return str("PANIC", l-LevelPanic)
	}
}

func parseLogLevel(s string) (l slog.Level, err error) {
	switch strings.ToUpper(s) {
	case "TRACE":
		return LevelTrace, nil
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	case "FATAL":
		return LevelFatal, nil
	case "PANIC":
		return LevelPanic, nil
	default:
		return l, l.UnmarshalText([]byte(s))
	}
}
