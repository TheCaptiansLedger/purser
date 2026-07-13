# TagAssignment — Giving `Tag` a Real Attachment Mechanism

> Status: proposal, not yet an ADR-backed decision. Conceptual, not Go
> structs — same convention as the module data-model docs. Written because
> `ports.TagRepository`'s own doc comment already flags the gap this
> resolves: *"Polymorphic attach/detach to an entity (item_tags/entry_tags/
> group_tags) is deferred."* `Tag` today is catalog-only CRUD (`Key`,
> `Value`, `Scope`, `Category`, per `internal/domain/tag.go`) — there is no
> way, in the running system, to actually tag anything.

## 1. What exists today vs. what was originally proposed

`domain.Tag` shipped exactly as `shared-domain-model.md` Part 1 specified
(`Tag{ ID, Key, Value, Scope, Category }`), but that same doc's plan for
the *attachment* side was never built:

> "Join tables unchanged from v1: `item_tags`, `entry_tags`, `group_tags`."
> — `shared-domain-model.md:89`

That's **three separate join tables**, one per attach-point level (Item,
LibraryEntry/"entry", Group). Since that doc was written, two other
polymorphic-attachment kernel types were actually designed and built —
`ExternalID{EntityType, EntityID, Source, Value}` and
`Image{OwnerType, OwnerID, ...}` — and both took the opposite approach:
**one generic type**, discriminated by an `EntityType`/`OwnerType` field,
not N separate per-level tables. Both are fully implemented today
(`internal/domain/external_id.go`, `internal/domain/image.go`,
`internal/ports/external_id.go`, `internal/ports/image.go`), persisted
through `internal/adapters/store` (`ExternalID` via the generic
`CompositeRepository[T]`, `Image` hand-written since its filter shape
differs — see `docs/adr/0012-datastore-persistence.md`'s addenda).

**This doc proposes following the pattern that actually got built, not
the one originally sketched**: one polymorphic `TagAssignment`, not three
separate join tables. Reasoning:

- **Consistency.** `Tag` would be the only polymorphic-attachment concept
  in the kernel still modeled as N separate tables while `ExternalID` and
  `Image` are both one generic type. Nothing about tags is structurally
  different from external IDs or images in this respect — all three
  attach one small record to "some entity, identified by type + id."
- **Zero new persistence-layer code.** `CompositeRepository[T]`
  (`internal/adapters/store/composite.go`) already exists, already proven
  end-to-end for `EntryPerson`, `ItemPerson`, and (via a thin
  type-converting wrapper) `ExternalID`. A polymorphic `TagAssignment`
  fits its exact shape — a new entity gets a wrapper package, not new
  generic machinery.
- **The cross-module payoff the original doc wanted still holds.** "Show
  me every movie, book, and scene tagged `genre:gonzo`" is still one join
  shape, not five — `EntityType` still lives on the attachment row, not
  on the tag, so the tag catalog itself stays module-agnostic either way.

## 2. Proposed shape

```
TagAssignment{ TagID, EntityType, EntityID }
```

No separate `ID` field — same convention as `EntryPerson`/`ItemPerson`/
`ExternalID`, whose identity *is* their natural key, not a generated one.
`EntityType` reuses the same `domain.EntityType` enum `ExternalID` and
`CollectionMembership` already use (`library_entry`, `group`, `item`,
`person`, extendable the same additive way those were) — not a
Tag-specific vocabulary.

A given `(TagID, EntityType, EntityID)` triple can only exist once
(re-tagging the same entity with the same tag is a no-op, not a
duplicate row) — same conflict semantics `Create` already gives every
other entity in this codebase.

## 3. Open question this doc does not resolve: which direction gets indexed

Real use cases (Part 4 of `shared-domain-model.md`) need **both**
directions to be efficient, not just one:

- **Browse-by-tag** ("everything tagged `genre:gonzo`", "filter the
  library by genre tag") — look up by `TagID`, get back entities.
- **Show an entity's tags** (a Scene or Album's detail page, tag chips) —
  look up by `EntityType`+`EntityID`, get back tags.

`CompositeRepository[T]` as it exists today only indexes 2 of a 3-part
key (`internal/adapters/store/composite.go`'s `index()` method) and its
`List` only filters on those same 2 fields — fine for `EntryPerson`/
`ItemPerson`/`ExternalID`, which only ever needed one filter direction in
practice. `TagAssignment` is the first case that plausibly needs both
directions indexed. Whoever implements this needs to decide between:

- Indexing all 3 fields independently (a small, generic enhancement to
  `CompositeRepository[T]` — index-everything, filter-on-any-subset —
  rather than a fixed k1/k2), benefiting any future 3-part-key entity
  with the same need, not just this one; or
- Building `TagAssignment` as its own hand-written translator (like
  `Image`) rather than forcing it through the existing generic, if the
  bidirectional-lookup need turns out to be `TagAssignment`-specific.

Not resolved here — flagged the same way `shared-domain-model.md` Part 5
flags its own open questions, for whoever picks this up.

## 4. What this unblocks (Part 4 use cases from `shared-domain-model.md`, currently blocked on this)

- Music #4: "Filter the library by genre tag." `Tag` join at `Group` level.
- AfterDark #1: "Query scenes by tag." `Item(ContentType=adult)` joined
  through the tag-assignment mechanism to `Tag(Key="genre", Value="gonzo")`.
- TV #4 / Books #5: same shape, `LibraryEntry` and `Group` level
  respectively.

All four are the same join shape at different attach points —
`EntityType` is what makes that true, per Section 1's reasoning.

## 5. Explicitly out of scope for this doc

- Proto/API design (request/response shapes, RPC names) — implementation
  detail, not a data-model decision.
- The List-filter work needed on `LibraryEntry`/`Group`/`Item` themselves
  (kind/parent/group filters) so browse-by-tag results can actually be
  paginated and displayed — related, tracked separately, not part of
  giving `Tag` an attachment mechanism.
- StashDB's tag `category`/`group` taxonomy (Section 3, Open Question 3
  of `afterdark-data_model.md`) — whether `Tag.Category` needs to grow
  structure to hold StashDB's two-level taxonomy is independent of
  whether tags can be attached to anything at all.
