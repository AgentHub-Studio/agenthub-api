# Integration Tests

Executa testes de integração dos commons com containers reais.

## Pré-requisitos
- Docker rodando

## Execução
```bash
cd /data/desenvolvimento/agenthub-middleware/agenthub-go-commons
docker run --rm \
  -v "$(pwd)":/app \
  -v "$HOME/go/pkg/mod":/go/pkg/mod \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -w /app \
  golang:1.24-alpine \
  go test -v -tags=integration ./integration/...
```
