# ADR-0002: Raw SQL with pgx

- Status: Accepted
- Context: The project needs PostgreSQL access with clear query control and no ORM layer.
- Decision: Use `pgx/v5` and `pgxpool` with explicit SQL strings.
- Consequences: More manual query code, but full control over indexing, constraints, and migration strategy.
