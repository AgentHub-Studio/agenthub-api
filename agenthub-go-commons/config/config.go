package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Common holds configuration shared by all AgentHub Go services.
type Common struct {
	DatabaseURL     string `env:"DATABASE_URL,required"`
	Port            int    `env:"PORT"          envDefault:"8080"`
	KeycloakBaseURL string `env:"KEYCLOAK_BASE_URL,required"`
	RabbitMQURL     string `env:"RABBITMQ_URL"  envDefault:"amqp://guest:guest@localhost:5672/"`
	OTLPEndpoint    string `env:"OTLP_ENDPOINT" envDefault:"http://localhost:4317"`
	LogLevel        string `env:"LOG_LEVEL"     envDefault:"info"`
}

// Load parses environment variables into a config struct T.
// T must embed or include fields with `env:` tags.
func Load[T any]() (T, error) {
	var cfg T
	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("config: parse env: %w", err)
	}
	return cfg, nil
}
