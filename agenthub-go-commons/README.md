# agenthub-go-commons

Módulos Go compartilhados entre os serviços AgentHub.

## Módulos

| Módulo | Responsabilidade |
|--------|-----------------|
| `auth/` | JWT middleware, cache JWKS por tenant (Keycloak) |
| `tenant/` | Multi-tenant middleware, `WithTenant()`, extração de tenantID do JWT |
| `database/` | pgxpool setup, health check, `AcquireWithTenant()` |
| `database/migrate/` | golang-migrate wrapper (compatível Flyway naming) |
| `database/multitenant/` | `MigrateAllTenants()` — itera schemas `ah_*` |
| `pagination/` | `Page[T]` genérico, `PageRequest` do query string |
| `httputil/` | Respostas HTTP padronizadas, binding, validação |
| `rabbitmq/` | Publisher/Consumer com retry e reconexão |
| `storage/` | MinIO wrapper — upload, download, presigned URLs |
| `config/` | Carregamento de env vars via struct tags (caarlos0/env) |
| `testutil/` | Testcontainers helpers, fixtures, migração para testes |
| `keycloak/` | Keycloak Admin API client — provisioning de realm, usuários, roles |
| `otel/` | OpenTelemetry setup — OTLP exporter, tracing, métricas |

## Uso

```go
import (
    "github.com/AgentHub-Studio/agenthub-go-commons/auth"
    "github.com/AgentHub-Studio/agenthub-go-commons/tenant"
    "github.com/AgentHub-Studio/agenthub-go-commons/database"
    "github.com/AgentHub-Studio/agenthub-go-commons/pagination"
)
```

## Build

```bash
./build.sh compile   # compila todos os pacotes
./build.sh test      # executa testes
./build.sh lint      # golangci-lint
./build.sh tidy      # go mod tidy
```

## Requisitos

- Go 1.24+
- Docker (builds via container — ADR-006)
