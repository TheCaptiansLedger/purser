# API Design

All functionality at `/api/v1/`. The React UI is just another API consumer.

## Standard CRUD

```
GET    /api/v1/{resource}          list with filter/sort/pagination
GET    /api/v1/{resource}/:id      single
POST   /api/v1/{resource}          create
PUT    /api/v1/{resource}/:id      full update
PATCH  /api/v1/{resource}/:id      partial update
DELETE /api/v1/{resource}/:id      delete
```

Resources: `library-entries`, `groups`, `items`, `people`, `tags`, `releases`, `downloads`, `commands`

## Async Operations

Long-running operations run as commands:

```
POST /api/v1/commands  { "name": "SearchItem", "ids": [...] }
GET  /api/v1/commands/:id
```

## Error Format

```json
{ "error": "human message", "code": "SNAKE_CASE_CODE" }
```

## Query Parameters

List endpoints support:

- `sort` — field name to sort by
- `sortDir` — `asc` or `desc`
- `page` — page number (1-based)
- `pageSize` — items per page
- `contentType` — filter by content type
- Additional resource-specific filters documented per endpoint
