# woodpantry-shopping-list — Shopping List Service

## Role in Architecture

Given a target set of recipes (or a meal plan), this service will produce a deduplicated, aggregated shopping list by diffing recipe ingredient requirements against current pantry state. The result should be grouped by ingredient category for easy grocery store navigation.

This service is **Phase 2+**. It is a read-heavy query service with light write responsibilities (persisting generated lists for retrieval). It does not own any ingredient or recipe data; it reads from Recipe Service, Pantry Service, and Ingredient Dictionary.

Current implementation status: the repository is scaffolded as a runnable Go service with config/env parsing, migrations, a health endpoint, tests, a Dockerfile, and Kubernetes manifests. The shopping-list generation and retrieval endpoints are still future work.

## Technology

- Language: Go
- HTTP: chi
- Database: PostgreSQL (`shopping_list_db`) via sqlc
- No RabbitMQ; this service is synchronous query-only

## Service Dependencies

- **Calls**: Recipe Service, Pantry Service, Ingredient Dictionary
- **Called by**: Web frontend
- **Publishes**: nothing
- **Subscribes to**: nothing

The current scaffold parses and validates `RECIPE_URL`, `PANTRY_URL`, and `DICTIONARY_URL`, but it does not yet perform outbound requests.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Implemented health check |
| POST | `/shopping-list` | Planned generation endpoint |
| GET | `/shopping-list/:id` | Planned retrieval endpoint |

Only `GET /healthz` is implemented in the current scaffold.

## Planned Behavior

### Generation Algorithm

```text
1. Fetch current pantry state from Pantry Service.
2. Fetch each requested recipe from Recipe Service.
3. Aggregate ingredient quantities across recipes.
4. Normalize units using Ingredient Dictionary conversion data.
5. Diff needed quantities against pantry stock.
6. Group remaining items by category.
7. Persist the generated list and return it.
```

### Fetch Strategy

Fetch all recipe details and pantry state before aggregation. Do not fetch ingredient metadata item-by-item in a tight loop when the next worker implements the algorithm.

## Data Models

The initial migration now creates:

```text
shopping_lists
  id              UUID  PK
  recipe_ids      UUID[]
  meal_plan_id    UUID  NULLABLE
  created_at      TIMESTAMPTZ

shopping_list_items
  id                    UUID  PK
  shopping_list_id      UUID  FK
  ingredient_id         UUID
  name                  TEXT
  category              TEXT
  quantity_needed       FLOAT8
  quantity_in_pantry    FLOAT8
  quantity_to_buy       FLOAT8
  unit                  TEXT
  created_at            TIMESTAMPTZ
```

CRUD query files are not implemented yet; `internal/db/queries/shopping_lists.sql` is intentionally just a placeholder so the sqlc layout is in place.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DB_URL` | required | PostgreSQL connection string for `shopping_list_db` |
| `RECIPE_URL` | required | Recipe Service base URL |
| `PANTRY_URL` | required | Pantry Service base URL |
| `DICTIONARY_URL` | required | Ingredient Dictionary base URL |
| `LOG_LEVEL` | `info` | Log level |

## Directory Layout

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
├── .mockery.yaml
├── Makefile
├── Dockerfile
├── go.mod
└── go.sum
```

`internal/clients`, generated sqlc code, handler implementations for shopping-list endpoints, and the actual generation logic are intentionally not present yet.

## What to Avoid

- Do not implement fake shopping-list generation just to fill files.
- Do not query another service's database directly.
- Do not add RabbitMQ.
- Do not replicate ingredient metadata as a source of truth in this service.
