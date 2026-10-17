# Load test

Run against a dev server on local Postgres 17 + Redis 7 (single API instance, 20 DB connections):

```
cd server && go run ./cmd/loadtest -base http://localhost:8080 -users 200 -duration 60s
```

Result (2026-10-17, laptop):

| Endpoint | Requests | p50 | p95 | p99 |
|---|---|---|---|---|
| GET /v1/bowls/:id/posts | 4,499 | 13.6 ms | 98.5 ms | 193.6 ms |
| GET /v1/conversations | 1,787 | 20.4 ms | 142.2 ms | 301.6 ms |
| GET /v1/feed | 7,731 | 19.6 ms | 145 ms | 280.5 ms |
| GET /v1/posts/:id/comments | 2,265 | 11 ms | 92.4 ms | 170.9 ms |
| POST /v1/conversations/:id/messages | 2,712 | 26.1 ms | 164.7 ms | 330.1 ms |
| POST /v1/posts/:id/comments | 1,658 | 20.1 ms | 120.9 ms | 237.7 ms |
| POST /v1/votes | 1,790 | 17.2 ms | 100.9 ms | 183 ms |

22,442 requests in 61 s (369 req/s), 0 errors. Feed p95 on 100k posts alone: 3.4 ms (T036).

Repeat on Railway staging before launch with the production instance size and `-users 500`.
