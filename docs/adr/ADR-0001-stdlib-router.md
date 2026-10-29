# ADR-0001: Standard library router

- Status: Accepted
- Context: The project needs a small, boring web stack with predictable behavior and no vendor lock-in.
- Decision: Use `net/http` and Go 1.22 routing patterns instead of a framework.
- Consequences: The application is slightly more verbose, but it remains fast, explicit, and easy to reason about.
