// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Compression    bool          `config:"compression.enabled"`
	Keepalives     bool          `config:"keepalives.enabled"`
	Redirects      bool          `config:"redirects.enabled"`
	RequestTimeout time.Duration `config:"request_timeout"`
}

var DefaultConfig = Config{
	Compression:    true,
	Keepalives:     true,
	Redirects:      true,
	RequestTimeout: 5 * time.Second,
}
