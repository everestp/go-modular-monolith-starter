# ADR-0004: Access + rotating refresh tokens

- Status: Accepted
- Context: The API needs short-lived access tokens and a safe way to keep users signed in.
- Decision: Use signed JWT access tokens plus opaque refresh tokens stored as hashes and rotated on every use.
- Consequences: The strategy is secure and easy to audit, but it requires careful token family cleanup and replay detection.
