# Proto Layout

Source of truth for the API layer's wire contracts. Generated Go code lands
in `gen/go/` (committed, not hand-edited — see `make proto-gen`). Full
rationale lives in [docs/adr/0011-api-design.md](../docs/adr/0011-api-design.md);
this file is just the directory convention.

```
proto/purser/domain/v1/*.proto      # one file per shared-kernel entity
                                     # (Person, LibraryEntry, Group, Item,
                                     # EntryPerson, ItemPerson, Tag,
                                     # ExternalID, Image, MediaFile)
proto/purser/afterdark/v1/*.proto   # AfterDark module-specific types
                                     # (PerformerProfile)
```

- One Connect service per file/entity — SRP at the proto level, matching
  `internal/service/**` and `internal/ports/**` (see
  [0001](../docs/adr/0001-hexagonal-architecture.md),
  [0002](../docs/adr/0002-solid-design-principles.md)).
- `v1` package suffix from day one; a breaking change to a service gets a
  `v2` package, not an edit to `v1`.
- Run `make proto-gen` after editing anything under `proto/` (requires
  `make tools` once beforehand — see the root `Makefile`).
