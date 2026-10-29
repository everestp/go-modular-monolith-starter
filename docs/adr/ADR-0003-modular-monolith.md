# ADR-0003: Modular monolith

- Status: Accepted
- Context: The team needs independent modules without paying the cost of a distributed system.
- Decision: Keep modules in a single process while enforcing package-level boundaries.
- Consequences: We can extract services later with less churn, while keeping operational simplicity.
