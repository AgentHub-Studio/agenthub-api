# _template — Template de projeto Go AgentHub

Copie estes arquivos para novos serviços Go:

- `Dockerfile` — multi-stage build (builder golang:1.24-alpine → scratch)
- `build.sh` — compile | test | package | lint | tidy (ADR-006)
- `.golangci.yml` — linters padrão

## Uso

```bash
cp agenthub-go-commons/_template/Dockerfile .
cp agenthub-go-commons/_template/build.sh .
cp agenthub-go-commons/_template/.golangci.yml .
chmod +x build.sh
```

## Build

```bash
SERVICE_NAME=agenthub-api ./build.sh compile
SERVICE_NAME=agenthub-api ./build.sh test
SERVICE_NAME=agenthub-api ./build.sh package
```
