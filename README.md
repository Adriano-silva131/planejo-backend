# planejo-backend

API do Planejo (planejoai.com) escrita em Go — substitui o backend Laravel do antigo EfficiencyTime.

## Rodando

```bash
cp .env.example .env
docker compose up -d --wait        # Postgres em localhost:5435
air                                # lê o .env, aplica as migrations e recompila a cada mudança
curl localhost:8080/health
```

Sem o air ([instalação](https://github.com/air-verse/air)): `set -a; source .env; set +a` e depois `go run ./cmd/api`.

Os testes de `internal/platform/db` sobem um Postgres com testcontainers, então precisam do Docker.
