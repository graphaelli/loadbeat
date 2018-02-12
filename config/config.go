// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import (
	"math"
	"time"
)

type Config struct {
	// Per Request Config
	Compression    bool          `config:"compression.enabled"`
	Keepalives     bool          `config:"keepalives.enabled"`
	Redirects      bool          `config:"redirects.enabled"`
	RequestTimeout time.Duration `config:"request_timeout"`

	// Work Config
	BaseUrls    []string       `config:"base_urls"`
	Targets     []TargetConfig `config:"targets"`
	MaxRequests int            `config:"max_requests"`
	RunTimeout  time.Duration  `config:"run_timeout"`
}

type TargetConfig struct {
	Body    string   `config:"body"`
	Headers []string `config:"headers"`
	Method  string   `config:"method"`
	Url     string   `config:"url"`

	Concurrent int     `config:"concurrent"`
	Qps        float64 `config:"qps"`
}

var DefaultConfig = Config{
	Compression:    true,
	Keepalives:     true,
	Redirects:      true,
	RequestTimeout: 5 * time.Second,

	BaseUrls:    []string{"http://apm-server:8200/"},
	MaxRequests: math.MaxInt32,
	RunTimeout:  1 * time.Minute,
}
