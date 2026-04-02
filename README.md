# woodpantry-shopping-list

Shopping List Service for WoodPantry. Given a set of recipe IDs, this service will eventually produce a deduplicated, aggregated shopping list diffed against current pantry state and grouped by ingredient category.

Current status: scaffolded Go service baseline for Phase 2. The repo now includes the service entrypoint, env parsing, DB migration bootstrap, Kubernetes manifests, and a health-only HTTP router. Shopping-list generation endpoints are not implemented yet.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Implemented health check |
| POST | `/shopping-list` | Planned; not implemented in the current scaffold |
| GET | `/shopping-list/:id` | Planned; not implemented in the current scaffold |

## Scaffolded Structure

```text
woodpantry-shopping-list/
├── cmd/shopping-list/main.go
├── internal/
│   ├── api/
│   │   ├── handlers.go
│   │   └── handlers_test.go
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   ├── migrations.go
│   │   └── sqlc.yaml
│   ├── logging/
│   │   └── logging.go
│   └── service/
│       └── service.go
├── kubernetes/
├── Dockerfile
├── Makefile
├── go.mod
└── .mockery.yaml
```

## Database Baseline

The initial migration creates:

```text
shopping_lists
  id
  recipe_ids
  meal_plan_id
  created_at

shopping_list_items
  id
  shopping_list_id
  ingredient_id
  name
  category
  quantity_needed
  quantity_in_pantry
  quantity_to_buy
  unit
  created_at
```

This is only the persistence scaffold. CRUD queries and generated `sqlc` output have not been added yet.

## Planned Generation Flow

1. Fetch pantry state from Pantry Service.
2. Fetch each requested recipe from Recipe Service.
3. Normalize and aggregate ingredient quantities using Dictionary conversion data.
4. Diff needed amounts against pantry stock.
5. Persist the generated list and return grouped results.

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DB_URL` | required | PostgreSQL `shopping_list_db` connection string |
| `RECIPE_URL` | required | Recipe Service base URL |
| `PANTRY_URL` | required | Pantry Service base URL |
| `DICTIONARY_URL` | required | Ingredient Dictionary base URL |
| `LOG_LEVEL` | `info` | Log level |

`PORT` and `LOG_LEVEL` default if unset. The service exits at startup if `DB_URL`, `RECIPE_URL`, `PANTRY_URL`, or `DICTIONARY_URL` are missing.

## Development Baseline

```bash
go test ./...
go build ./cmd/shopping-list
go run ./cmd/shopping-list
sqlc generate -f internal/db/sqlc.yaml
```

At this stage, `go run` starts a service that validates config, connects to Postgres, runs migrations, and serves `GET /healthz`.
