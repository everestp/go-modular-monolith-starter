# ADR-0005: RFC 9457 problem details

- Status: Accepted
- Context: APIs must expose consistent, machine-readable errors without leaking internal stack traces.
- Decision: Return RFC 9457 `application/problem+json` payloads with request IDs and field details.
- Consequences: Clients gain a predictable error surface, and the server keeps internal exceptions private.
