# planejo-backend

API do Planejo (planejoai.com) escrita em Go — substitui o backend Laravel do antigo EfficiencyTime.

## Rodando

```bash
cp .env.example .env
docker compose up -d --wait        # Postgres em localhost:5435
set -a; source .env; set +a
go run ./cmd/api                   # aplica as migrations pendentes ao subir
curl localhost:8080/health
```

Os testes de `internal/platform/db` sobem um Postgres com testcontainers, então precisam do Docker.
