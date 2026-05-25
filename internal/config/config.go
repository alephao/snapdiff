// Package config loads and validates the per-project snapdiff.toml.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Snapshots SnapshotsConfig
	Server    ServerConfig
}

type SnapshotsConfig struct {
	Globs     []string
	AxisRegex *regexp.Regexp
	BaseRef   string
}

type ServerConfig struct {
	Bind          string
	LingerSeconds int
}

type rawConfig struct {
	Snapshots rawSnapshots `toml:"snapshots"`
	Server    rawServer    `toml:"server"`
}

type rawSnapshots struct {
	Globs     []string `toml:"globs"`
	AxisRegex string   `toml:"axis_regex"`
	BaseRef   string   `toml:"base_ref"`
}

type rawServer struct {
	Bind          string `toml:"bind"`
	LingerSeconds *int   `toml:"linger_seconds"`
}

const (
	defaultBaseRef       = "HEAD"
	defaultBind          = "0.0.0.0:7777"
	defaultLingerSeconds = 60
)

// Load reads and validates the snapdiff config at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if len(raw.Snapshots.Globs) == 0 {
		return nil, errors.New("snapshots.globs must contain at least one glob")
	}
	if raw.Snapshots.AxisRegex == "" {
		return nil, errors.New("snapshots.axis_regex is required")
	}

	rx, err := regexp.Compile(raw.Snapshots.AxisRegex)
	if err != nil {
		return nil, fmt.Errorf("snapshots.axis_regex is not a valid regexp: %w", err)
	}

	baseRef := raw.Snapshots.BaseRef
	if baseRef == "" {
		baseRef = defaultBaseRef
	}

	bind := raw.Server.Bind
	if bind == "" {
		bind = defaultBind
	}

	linger := defaultLingerSeconds
	if raw.Server.LingerSeconds != nil {
		linger = *raw.Server.LingerSeconds
	}
	if linger < 0 {
		return nil, fmt.Errorf("server.linger_seconds must be >= 0, got %d", linger)
	}

	return &Config{
		Snapshots: SnapshotsConfig{
			Globs:     raw.Snapshots.Globs,
			AxisRegex: rx,
			BaseRef:   baseRef,
		},
		Server: ServerConfig{
			Bind:          bind,
			LingerSeconds: linger,
		},
	}, nil
}
