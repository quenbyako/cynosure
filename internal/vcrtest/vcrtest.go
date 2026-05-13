package vcrtest

import (
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
	"gopkg.in/yaml.v3"
)

// CassetteTTL is the default time-to-live for a VCR cassette.
const (
	CassetteTTL = 30 * 24 * time.Hour
)

type cassetteWrapper struct {
	RecordedAt time.Time          `yaml:"recorded_at"`
	Cassette   *cassette.Cassette `yaml:",inline"`
}

func customMarshal(v any) ([]byte, error) {
	cas, ok := v.(*cassette.Cassette)
	if !ok {
		return yaml.Marshal(v)
	}
	wrapper := cassetteWrapper{
		RecordedAt: time.Now().UTC(),
		Cassette:   cas,
	}
	return yaml.Marshal(wrapper)
}

// New creates a new recorder with the given cassette name, current mode, and custom options.
func New(t *testing.T, fsys fs.FS, name string, opts ...recorder.Option) *recorder.Recorder {
	t.Helper()

	mode := Mode(t)
	cassFile := name + ".yaml"

	// Check expiration directly from the YAML file
	if data, err := fs.ReadFile(fsys, cassFile); err == nil {
		var meta struct {
			RecordedAt time.Time `yaml:"recorded_at"`
		}
		if yamlErr := yaml.Unmarshal(data, &meta); yamlErr == nil &&
			!meta.RecordedAt.IsZero() &&
			time.Since(meta.RecordedAt) > CassetteTTL &&
			mode == recorder.ModeReplayOnly {
			t.Fatalf("VCR cassette %s is expired (> %s, recorded at: %s). Run with -tags=vcr_record to update.", name, CassetteTTL, meta.RecordedAt.Format(time.RFC3339))
		}
	} else if mode == recorder.ModeReplayOnly {
		t.Fatalf("VCR cassette %s not found in replay mode: %v", cassFile, err)
	}

	matcher := cassette.NewDefaultMatcher(
		cassette.WithIgnoreAuthorization(),
		cassette.WithIgnoreHeaders("X-Goog-Api-Key", "Set-Cookie"),
	)

	// Combine default mode and custom marshaler with user-provided options
	finalOpts := append([]recorder.Option{
		recorder.WithMode(mode),
		recorder.WithMarshalFunc(customMarshal),
		recorder.WithMatcher(matcher),
	}, opts...)

	r, err := recorder.New(name, finalOpts...)
	require.NoError(t, err)

	return r
}
