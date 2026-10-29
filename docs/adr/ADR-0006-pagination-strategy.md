# ADR-0006: Keyset pagination

- Status: Accepted
- Context: The notes API needs efficient paging and ordering without expensive offset scans.
- Decision: Use cursor-based keyset pagination with a safe whitelist of sort fields.
- Consequences: The API scales better but requires clients to follow the cursor explicitly.
