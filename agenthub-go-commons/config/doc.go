// Package config provides environment variable loading via caarlos0/env struct tags.
// Each AgentHub service embeds the common config and adds service-specific fields.
//
// Usage:
//
//	type Config struct {
//	    config.Common
//	    Port int `env:"PORT" envDefault:"8080"`
//	}
//	cfg, err := config.Load[Config]()
package config
