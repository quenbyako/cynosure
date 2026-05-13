// Package vcrtest provides utilities for VCR-based integration testing.
package vcrtest

import (
	"fmt"
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
	//nolint:tagliatelle // Saving same style as in vcr library
	RecordedAt time.Time          `yaml:"recorded_at"`
	Cassette   *cassette.Cassette `yaml:",inline"`
}

func customMarshal(v any) ([]byte, error) {
	cas, ok := v.(*cassette.Cassette)
	if !ok {
		res, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshalling cassette: %w", err)
		}

		return res, nil
	}

	wrapper := cassetteWrapper{
		RecordedAt: time.Now().UTC(),
		Cassette:   cas,
	}

	res, err := yaml.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("yaml marshal wrapper: %w", err)
	}

	return res, nil
}

// New creates a new recorder with the given cassette name, current mode, and custom options.
func New(t *testing.T, fsys fs.FS, name string, opts ...recorder.Option) *recorder.Recorder {
	t.Helper()

	mode := Mode(t)
	checkCassetteExpiration(t, fsys, name, mode)

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

func checkCassetteExpiration(t *testing.T, fsys fs.FS, name string, mode recorder.Mode) {
	t.Helper()

	cassFile := name + ".yaml"

	data, err := fs.ReadFile(fsys, cassFile)
	if err != nil {
		if mode == recorder.ModeReplayOnly {
			t.Fatalf("VCR cassette %s not found in replay mode: %v", cassFile, err)
		}

		return
	}

	var meta struct {
		//nolint:tagliatelle // Using snake_case for compatibility with existing cassettes
		RecordedAt time.Time `yaml:"recorded_at"`
	}

	if err := yaml.Unmarshal(data, &meta); err != nil {
		return
	}

	if isExpired(meta.RecordedAt, mode) {
		t.Fatalf(
			"VCR cassette %s is expired (> %s, recorded at: %s). "+
				"Run with -tags=vcr_record to update.",
			name,
			CassetteTTL,
			meta.RecordedAt.Format(time.RFC3339),
		)
	}
}

func isExpired(recordedAt time.Time, mode recorder.Mode) bool {
	if recordedAt.IsZero() {
		return false
	}

	if mode != recorder.ModeReplayOnly {
		return false
	}

	return time.Since(recordedAt) > CassetteTTL
}
