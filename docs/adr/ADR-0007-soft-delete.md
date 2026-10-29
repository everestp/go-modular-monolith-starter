# ADR-0007: Soft delete for notes

- Status: Accepted
- Context: Notes are user-owned content and can be restored or audited later.
- Decision: Keep a nullable `deleted_at` column and filter soft-deleted rows by default.
- Consequences: Queries need a `deleted_at IS NULL` guard, but the design gives operational safety and simpler recovery.
