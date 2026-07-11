# 0002. SOLID Design Principles

Status: Accepted

## Context

"SOLID" is easy to cite and easy to violate in ways that only surface once a
second or third content type is added. This ADR pins down what each letter
means *in this codebase specifically*, so it can be checked mechanically
instead of debated abstractly.

## Decision

- **S — Single Responsibility.** A service, handler, or adapter has exactly
  one reason to change. If a change to "how we talk to MusicBrainz" requires
  editing the same function as a change to "how we render the album page,"
  that function has more than one responsibility.
- **O — Open/Closed.** Adding a new content type, a new metadata source, or a
  new capability must be done by adding new code (a new adapter, a new
  config entry), not by editing an existing, already-tested function. A type
  switch over content types in shared code is a standing OCP violation — see
  [0001](0001-hexagonal-architecture.md).
- **L — Liskov Substitution.** Any adapter implementing a port must be
  substitutable for any other implementation of that port without the
  caller needing to know which one it got. If calling code has special
  handling for "if this is the BadgerDB adapter, do X," the port's contract
  is incomplete or the special case doesn't belong there.
- **I — Interface Segregation.** Ports are narrow and capability-specific
  (`ImageSource`, `MetadataSource`, `TrackRepository`) rather than one large
  `Provider` interface every adapter must fully implement. An adapter that
  doesn't support a capability simply doesn't implement that narrow
  interface — callers check for the capability, not for a "not supported"
  branch inside a monolithic interface.
- **D — Dependency Inversion.** Services depend on port interfaces declared
  in the domain/service layer, not on adapter packages. Adapter packages
  depend on port interfaces too (by implementing them) — the dependency
  arrow always points at the interface, never at a concrete package name.

## Consequences

- Every new port is a design decision, not a formality — narrow interfaces
  mean more of them, and each one needs a clear reason to exist.
- Violating any of the five here is equivalent to violating the hexagonal
  boundary in [0001](0001-hexagonal-architecture.md) — the two ADRs are
  checked together, not separately.

## Self-Audit Checklist

Run after every task that adds or changes a service, port, or adapter:

1. **SRP** — Can I describe this function/type's reason to change in one
   sentence? If it takes "and," split it.
2. **OCP** — Did adding this feature require editing an existing, working
   function's logic (not just registering something new)? If yes — fix it.
3. **LSP** — Does any caller type-switch or type-assert on a concrete adapter
   type instead of using it through its port interface? If yes — fix it.
4. **ISP** — Does any port interface have methods that most implementers
   have to stub out or return `ErrNotSupported` from? If yes — split the
   interface.
5. **DIP** — Does any service or domain file import an adapter package
   directly (not just the port interface)? If yes — fix it.
