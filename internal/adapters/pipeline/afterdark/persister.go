// This file is the ports.Persister implementation for
// domain.ContentTypeAdult — AD7 (issue #561), mirroring
// internal/adapters/pipeline/music/persister.go's shape: get-or-create
// Studio -> Performers -> Scene + MediaFile, reusing
// docs/adr/0026-external-id-get-or-create.md's speculative-create-then-
// reconcile pattern. Unlike Music (always one candidate, per
// docs/adr/0025), candidates here is 1..N — one per provider that
// independently cleared threshold, per
// docs/adr/0027-provider-independence.md — so this file additionally
// unions tags/ExternalIDs/performer-credits across every candidate, and
// resolves Title/Overview/Date/Studio conflicts via a user-configured
// provider priority order (config.AfterDark.ProviderPriority) rather than
// picking a winner itself. Scene image attachment (AD9, issue #563) unions
// every candidate's own poster/screenshot images the same way, writing
// bytes via ports.ImageFetcher + ports.ImageStore
// (docs/adr/0013-image-blob-storage.md) before the ports.ImageRepository
// row that points at them.
package afterdark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// performerRole is the ItemPerson.Role every AfterDark scene credit is
// created with — per docs/technical/afterdark-data_model.md §5's Performer
// mapping ("performer"/"actress" collapse to one role vocabulary; "actress"
// was v1's JAV-specific term, not carried forward as a separate value here).
const performerRole = "performer"

// tagKey is the flat domain.Tag.Key every AfterDark scene/JAV tag is
// created under. StashDB's Tag/ThePornDB's TPDBTag DTOs (internal/ports)
// carry no category/group structure — AD1/AD2 never fetched it — so
// docs/technical/afterdark-data_model.md §5 decision 4's
// key="category:group" flattening has nothing to flatten from; every tag
// name becomes Tag{Key: tagKey, Value: name}. Worth revisiting if a future
// adapter pass adds StashDB's category/group fields.
const tagKey = "tag"

// sceneOwnerType is the Image.OwnerType value every AfterDark scene image
// (AD9, issue #563) is attached under. Unlike Music's own
// musicReleaseOwnerType ("music_release", a module-specific type), a Scene
// *is* the shared kernel domain.Item directly, so this is
// domain.EntityTypeItem's own string value — same value
// getOrCreateScene/persistTags already key ExternalID/TagAssignment rows
// against for the same Item.
const sceneOwnerType = string(domain.EntityTypeItem)

// Persister implements ports.Persister for domain.ContentTypeAdult. See
// this file's package comment.
type Persister struct {
	stashDB ports.StashDBClient
	tpdb    ports.ThePornDBClient

	externalIDs       ports.ExternalIDRepository
	libraryEntries    ports.LibraryEntryRepository
	persons           ports.PersonRepository
	performerProfiles ports.PerformerProfileRepository
	itemPeople        ports.ItemPersonRepository
	items             ports.ItemRepository
	mediaFiles        ports.MediaFileRepository
	tags              ports.TagRepository
	tagAssignments    ports.TagAssignmentRepository
	images            ports.ImageRepository
	imageStore        ports.ImageStore
	imageFetcher      ports.ImageFetcher
	organizer         ports.Organizer

	// providerPriority is config.AfterDark.ProviderPriority — see
	// selectPrimary.
	providerPriority []string

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.Persister = (*Persister)(nil)

// NewPersister constructs a Persister. organizer is called after every
// MediaFile this Persister creates, same "inject a Noop, never nil-check an
// optional port" convention pipelinemusic.NewPersister's doc comment
// describes — always non-nil. images/imageStore/imageFetcher are AD9's
// (issue #563) scene-image attachment step, same three-port shape
// pipelinemusic.NewPersister's own images/imageStore params use for cover
// art, plus imageFetcher (docs/adr/0013-image-blob-storage.md's Addendum)
// to turn a provider's image URL into bytes ImageStore.Put can write —
// Music's local-sidecar cover art never needed that extra step.
// providerPriority is config.AfterDark.ProviderPriority, read once at
// construction (a config reload requires a process restart, same as every
// other pipeline config value). Reuses the same
// Option/WithLogger/WithTracerProvider declared in fingerprinter.go.
func NewPersister(
	stashDB ports.StashDBClient,
	tpdb ports.ThePornDBClient,
	externalIDs ports.ExternalIDRepository,
	libraryEntries ports.LibraryEntryRepository,
	persons ports.PersonRepository,
	performerProfiles ports.PerformerProfileRepository,
	itemPeople ports.ItemPersonRepository,
	items ports.ItemRepository,
	mediaFiles ports.MediaFileRepository,
	tags ports.TagRepository,
	tagAssignments ports.TagAssignmentRepository,
	images ports.ImageRepository,
	imageStore ports.ImageStore,
	imageFetcher ports.ImageFetcher,
	organizer ports.Organizer,
	providerPriority []string,
	opts ...Option,
) *Persister {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Persister{
		stashDB:           stashDB,
		tpdb:              tpdb,
		externalIDs:       externalIDs,
		libraryEntries:    libraryEntries,
		persons:           persons,
		performerProfiles: performerProfiles,
		itemPeople:        itemPeople,
		items:             items,
		mediaFiles:        mediaFiles,
		tags:              tags,
		tagAssignments:    tagAssignments,
		images:            images,
		imageStore:        imageStore,
		imageFetcher:      imageFetcher,
		organizer:         organizer,
		providerPriority:  providerPriority,
		logger:            o.logger.With("component", "adapters.pipeline.afterdark.persister"),
		tracer:            o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.Persister.
func (p *Persister) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// resolvedStudio is a candidate's own studio/site info, normalized from
// either provider's DTO shape.
type resolvedStudio struct {
	source      string
	externalRef string
	name        string
}

// resolvedPerformer is one scene-credit's performer info, normalized from
// either provider's DTO shape. For a ThePornDB site-specific alias
// performer record (Parent != nil), externalRef is always the canonical
// parent's own ID, never the alias record's — ExternalID's storage
// identity is the composite (entityType, entityID, source)
// (internal/ports/external_id.go), so a Person can only carry one
// tpdb-sourced value; the alias's own ID is deliberately not linked
// separately (a real, narrower reading of
// docs/technical/afterdark-data_model.md §5 decision 5's "its own
// ThePornDB ID is also linked via its own ExternalID row" than that text
// literally describes — not achievable under the current schema without a
// second source value, which this issue doesn't introduce). The alias's
// display name still folds into aliases, so it's discoverable either way;
// re-scans converge on the same Person because resolveTPDBPerformer always
// resolves externalRef to the canonical ID regardless of which record
// (alias or canonical) a future scene response returns it through.
type resolvedPerformer struct {
	source      string
	externalRef string

	name       string
	creditedAs string
	aliases    []string

	gender      domain.Gender
	birthDate   *time.Time
	nationality string
	overview    string

	cupSize         string
	bandSize        string
	breastType      string
	careerStartYear int
	careerEndYear   int
}

// resolvedImage is one image URL a candidate's provider returned for the
// scene — width/height are StashDB-only (its Image DTO carries them;
// ThePornDB's plain URL strings don't), left zero when unknown.
type resolvedImage struct {
	url    string
	width  int
	height int
}

// resolvedCandidate is one candidate's full scene detail, normalized from
// either provider's DTO shape — Identify only populated ExternalRef/Title/
// Tier/Signals, so resolveCandidateScene calls back into the owning
// provider (candidate.Metadata["source"]) for everything else this cascade
// needs.
type resolvedCandidate struct {
	source      string
	externalRef string

	title    string
	overview string
	date     *time.Time
	isJAV    bool
	javCode  string

	studio     *resolvedStudio
	performers []resolvedPerformer
	tags       []string
	images     []resolvedImage
}

// Persist implements ports.Persister. candidates is never empty (per the
// port's own doc comment); every candidate is resolved to full scene detail
// independently, tags/ExternalIDs/performer credits are unioned across all
// of them, and Title/Overview/Date/Studio (single-valued — an Item has one
// LibraryEntryID) are taken from whichever candidate's source appears
// earliest in providerPriority. A candidate whose own LookupScene call
// fails is logged and dropped, not fatal — mirrors Identify's own
// per-provider error tolerance (docs/adr/0027-provider-independence.md: a
// StashDB outage must not block a ThePornDB-only result, or vice versa); an
// error is only returned once every candidate has failed.
func (p *Persister) Persist(ctx context.Context, _ *domain.Fingerprint, candidates []domain.MatchCandidate, files []*domain.UnmatchedFile) error {
	ctx, span := p.tracer.Start(ctx, "afterdark.persister.persist", trace.WithAttributes(
		attribute.Int("afterdark.candidate_count", len(candidates)),
	))
	defer span.End()

	resolved := make([]resolvedCandidate, 0, len(candidates))
	for _, c := range candidates {
		rc, err := p.resolveCandidateScene(ctx, c)
		if err != nil {
			p.logger.WarnContext(ctx, "resolving candidate scene failed, dropping candidate", "external_ref", c.ExternalRef, "error", err)
			continue
		}
		resolved = append(resolved, rc)
	}
	if len(resolved) == 0 {
		return fmt.Errorf("adapters/pipeline/afterdark: no candidate could be resolved against its provider")
	}

	primary := p.selectPrimary(resolved)

	studioID, err := p.getOrCreateStudio(ctx, primary)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/afterdark: resolving studio: %w", err)
	}

	item, err := p.getOrCreateScene(ctx, resolved, primary, studioID)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/afterdark: resolving scene: %w", err)
	}

	if len(files) > 0 {
		if err := p.persistMediaFile(ctx, item, files[0]); err != nil {
			return fmt.Errorf("adapters/pipeline/afterdark: persisting media file for scene %s: %w", item.ID, err)
		}
	}

	if err := p.persistPerformers(ctx, item.ID, resolved); err != nil {
		return fmt.Errorf("adapters/pipeline/afterdark: persisting performers for scene %s: %w", item.ID, err)
	}

	if err := p.persistTags(ctx, item.ID, resolved); err != nil {
		return fmt.Errorf("adapters/pipeline/afterdark: persisting tags for scene %s: %w", item.ID, err)
	}

	if err := p.persistSceneImages(ctx, item, resolved); err != nil {
		return fmt.Errorf("adapters/pipeline/afterdark: persisting images for scene %s: %w", item.ID, err)
	}

	p.logger.InfoContext(ctx, "afterdark scene persisted",
		"item.id", item.ID, "item.title", item.Title, "candidate_count", len(resolved))
	return nil
}

// resolveCandidateScene dispatches to c's own provider (Metadata["source"],
// set by every tier in identifier.go) for the full scene detail Identify
// itself never populated.
func (p *Persister) resolveCandidateScene(ctx context.Context, c domain.MatchCandidate) (resolvedCandidate, error) {
	source, _ := c.Metadata["source"].(string)
	switch source {
	case sourceStashDB:
		scene, err := p.stashDB.LookupScene(ctx, c.ExternalRef)
		if err != nil {
			return resolvedCandidate{}, err
		}
		return resolveStashDBScene(*scene, c), nil
	case sourceTPDB:
		scene, err := p.tpdb.LookupScene(ctx, c.ExternalRef)
		if err != nil {
			return resolvedCandidate{}, err
		}
		return resolveTPDBScene(*scene, c), nil
	default:
		return resolvedCandidate{}, fmt.Errorf("candidate %s has unrecognized or missing source %q", c.ExternalRef, source)
	}
}

// selectPrimary returns the resolved candidate whose source appears
// earliest in p.providerPriority — the single source Studio/Title/Overview/
// Date are taken from. Falls back to resolved[0] (preserving the order
// Persist was called with, never re-ranking) when providerPriority is empty
// or none of its entries match any cleared candidate's source, the same
// "no configured preference, use given order" fallback DecisionService's
// own doc comment describes for candidate ordering in general.
func (p *Persister) selectPrimary(resolved []resolvedCandidate) resolvedCandidate {
	for _, source := range p.providerPriority {
		for _, rc := range resolved {
			if rc.source == source {
				return rc
			}
		}
	}
	return resolved[0]
}

// externalIDSourceFor maps an internal source tag ("stashdb"/"tpdb", per
// identifier.go's sourceStashDB/sourceTPDB) to its domain.ExternalIDSource.
// Only ever called with a resolvedCandidate/resolvedPerformer/
// resolvedStudio's own source field, which resolveCandidateScene's switch
// already restricts to these two values.
func externalIDSourceFor(source string) domain.ExternalIDSource {
	if source == sourceStashDB {
		return domain.ExternalIDSourceStashDB
	}
	return domain.ExternalIDSourceTPDB
}

// resolveOrCreate implements ADR-0026's speculative-create-then-reconcile
// pattern generically — mirrors music.Persister's own resolveOrCreate
// (docs/adr/0026-external-id-get-or-create.md). Duplicated rather than
// shared: each pipeline content-type package stays self-contained, the same
// convention this package's nameSimilarity/FilenameParser duplication
// already follows.
func (p *Persister) resolveOrCreate(
	ctx context.Context,
	entityType domain.EntityType,
	source domain.ExternalIDSource,
	value string,
	createSpeculative func(ctx context.Context) (id string, err error),
	deleteByID func(ctx context.Context, id string) error,
) (string, error) {
	existing, err := p.externalIDs.GetByValue(ctx, entityType, source, value)
	if err == nil {
		return existing.EntityID, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return "", err
	}

	id, err := createSpeculative(ctx)
	if err != nil {
		return "", err
	}

	link := &domain.ExternalID{EntityType: entityType, EntityID: id, Source: source, Value: value}
	if err := p.externalIDs.Create(ctx, link); err != nil {
		return "", err
	}
	if link.EntityID != id {
		if delErr := deleteByID(ctx, id); delErr != nil {
			return "", fmt.Errorf("adapters/pipeline/afterdark: cleaning up speculative entity %s after losing get-or-create race: %w", id, delErr)
		}
		return link.EntityID, nil
	}
	return id, nil
}

// linkAdditionalExternalID attaches an extra (source, value) identity onto
// an already-resolved entityID — the mechanism behind this file's "union"
// steps: a Scene ends up with one ExternalID per cleared candidate's own
// provider (plus jav_code), a ThePornDB alias performer's own ID gets
// linked alongside its canonical parent's. If (entityType, source, value)
// is already linked to a *different* entity, that's a genuine data
// conflict this call didn't create and isn't positioned to resolve — logged
// and left as-is, the same "log and skip, never abort the rest of Persist"
// tolerance Identify already gives a single provider's failure.
func (p *Persister) linkAdditionalExternalID(ctx context.Context, entityType domain.EntityType, entityID string, source domain.ExternalIDSource, value string) error {
	if value == "" {
		return nil
	}
	link := &domain.ExternalID{EntityType: entityType, EntityID: entityID, Source: source, Value: value}
	if err := link.Validate(); err != nil {
		return err
	}
	if err := p.externalIDs.Create(ctx, link); err != nil {
		return err
	}
	if link.EntityID != entityID {
		p.logger.WarnContext(ctx, "external id already linked to a different entity, leaving as-is",
			"entity_type", entityType, "entity_id", entityID, "source", source, "value", value, "existing_entity_id", link.EntityID)
	}
	return nil
}

// getOrCreateStudio resolves primary's studio to a LibraryEntry.ID, per
// docs/technical/afterdark-data_model.md §5's Studio mapping — get-or-
// create via ExternalID(library_entry, source, studioRef). Only the
// priority-selected primary candidate's studio is used: an Item has one
// LibraryEntryID, so unlike tags/performers there is nothing to union here.
func (p *Persister) getOrCreateStudio(ctx context.Context, primary resolvedCandidate) (string, error) {
	if primary.studio == nil {
		return "", fmt.Errorf("candidate %s has no studio", primary.externalRef)
	}
	studio := primary.studio

	return p.resolveOrCreate(ctx, domain.EntityTypeLibraryEntry, externalIDSourceFor(studio.source), studio.externalRef, func(ctx context.Context) (string, error) {
		entry := &domain.LibraryEntry{
			ID:          domain.NewID(),
			ContentType: domain.ContentTypeAdult,
			Kind:        domain.KindStudio,
			Name:        studio.name,
			MonitorMode: domain.MonitorModeAll,
		}
		if err := entry.Validate(); err != nil {
			return "", err
		}
		if err := p.libraryEntries.Create(ctx, entry); err != nil {
			return "", err
		}
		return entry.ID, nil
	}, p.libraryEntries.Delete)
}

// sceneKind returns Item.Metadata["kind"] per docs/technical/afterdark-data_model.md
// §5 decision 1: "jav_title" if any resolved candidate carries a JAV code,
// "scene" otherwise.
func sceneKind(resolved []resolvedCandidate) string {
	for _, rc := range resolved {
		if rc.isJAV {
			return "jav_title"
		}
	}
	return "scene"
}

// javCodeOf returns the first non-empty javCode among resolved, or "" if
// none carries one.
func javCodeOf(resolved []resolvedCandidate) string {
	for _, rc := range resolved {
		if rc.javCode != "" {
			return rc.javCode
		}
	}
	return ""
}

// getOrCreateScene resolves the Item this candidate set identifies,
// unioning every candidate's own (source, externalRef) — and jav_code, if
// any candidate resolved one — onto it, whether it was just created or
// already existed. Checked in that order (per-candidate provider IDs, then
// jav_code) since a provider ID is this content's most direct identity; the
// jav_code fallback exists specifically for the case a prior import only
// linked a different provider's ID (docs/domain/external_id.go's
// ExternalIDSourceJAVCode doc comment).
func (p *Persister) getOrCreateScene(ctx context.Context, resolved []resolvedCandidate, primary resolvedCandidate, studioID string) (*domain.Item, error) {
	javCode := javCodeOf(resolved)

	item, err := p.findExistingScene(ctx, resolved, javCode)
	switch {
	case err == nil:
		if err := p.reconcileSceneScalars(ctx, item, resolved, primary); err != nil {
			return nil, err
		}
	case errors.Is(err, ports.ErrNotFound):
		item, err = p.createScene(ctx, resolved, primary, studioID)
		if err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	for _, rc := range resolved {
		if err := p.linkAdditionalExternalID(ctx, domain.EntityTypeItem, item.ID, externalIDSourceFor(rc.source), rc.externalRef); err != nil {
			return nil, err
		}
	}
	if javCode != "" {
		if err := p.linkAdditionalExternalID(ctx, domain.EntityTypeItem, item.ID, domain.ExternalIDSourceJAVCode, javCode); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// findExistingScene looks up an already-persisted Item via any of
// resolved's own provider links, then javCode — returns ports.ErrNotFound
// (never a bare nil, nil) if none hit, so getOrCreateScene's caller can
// switch on the sentinel the same way every other get-or-create lookup in
// this codebase does.
func (p *Persister) findExistingScene(ctx context.Context, resolved []resolvedCandidate, javCode string) (*domain.Item, error) {
	for _, rc := range resolved {
		existing, err := p.externalIDs.GetByValue(ctx, domain.EntityTypeItem, externalIDSourceFor(rc.source), rc.externalRef)
		if err == nil {
			return p.items.Get(ctx, existing.EntityID)
		}
		if !errors.Is(err, ports.ErrNotFound) {
			return nil, err
		}
	}
	if javCode != "" {
		existing, err := p.externalIDs.GetByValue(ctx, domain.EntityTypeItem, domain.ExternalIDSourceJAVCode, javCode)
		if err == nil {
			return p.items.Get(ctx, existing.EntityID)
		}
		if !errors.Is(err, ports.ErrNotFound) {
			return nil, err
		}
	}
	return nil, ports.ErrNotFound
}

// createScene speculatively creates a new Item and establishes it as the
// winner (or loser, per ADR-0026) of a concurrent get-or-create race keyed
// on resolved[0]'s own (source, externalRef) — the same "first candidate
// decides the race, remaining candidates' links attach after" shape
// getOrCreateScene's caller uses for the union step.
func (p *Persister) createScene(ctx context.Context, resolved []resolvedCandidate, primary resolvedCandidate, studioID string) (*domain.Item, error) {
	item := &domain.Item{
		ID:             domain.NewID(),
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: studioID,
		Title:          primary.title,
		Overview:       primary.overview,
		Date:           primary.date,
		Status:         domain.ItemStatusMissing,
		Metadata:       map[string]any{"kind": sceneKind(resolved)},
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}

	first := resolved[0]
	winnerID, err := p.resolveOrCreate(ctx, domain.EntityTypeItem, externalIDSourceFor(first.source), first.externalRef, func(ctx context.Context) (string, error) {
		if err := p.items.Create(ctx, item); err != nil {
			return "", err
		}
		return item.ID, nil
	}, p.items.Delete)
	if err != nil {
		return nil, err
	}
	if winnerID == item.ID {
		return item, nil
	}
	return p.items.Get(ctx, winnerID)
}

// datesEqual compares two possibly-nil *time.Time values.
func datesEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// reconcileSceneScalars updates an already-persisted scene's scalar fields
// to match a fresh Persist call's freshly-computed primary/kind — a rescan
// can change which provider is primary (a newly-cleared higher-priority
// candidate, or a changed config.AfterDark.ProviderPriority) or which
// Metadata["kind"] applies (a JAV code newly resolved this pass).
// Studio/LibraryEntryID is deliberately left unchanged on an existing
// scene — moving an already-linked scene to a different Studio on a later
// rescan is out of scope here, the same simplification Music's own
// getOrCreateRelease/getOrCreateReleaseGroup retry path makes (idempotent
// identity resolution, not a general re-sync).
func (p *Persister) reconcileSceneScalars(ctx context.Context, item *domain.Item, resolved []resolvedCandidate, primary resolvedCandidate) error {
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	kind := sceneKind(resolved)
	changed := item.Title != primary.title || item.Overview != primary.overview || !datesEqual(item.Date, primary.date) || item.Metadata["kind"] != kind
	if !changed {
		return nil
	}
	item.Title = primary.title
	item.Overview = primary.overview
	item.Date = primary.date
	item.Metadata["kind"] = kind
	return p.items.Update(ctx, item)
}

// persistMediaFile implements the same "already known" retry-safety shape
// as music.Persister.persistOneTrack: if a MediaFile already exists for
// uf's hashes, only reconciles the linked Item's Status; otherwise creates
// the MediaFile, flips item's Status to imported, and best-effort calls the
// Organizer (an error is logged and swallowed, never returned — organizing
// is a bonus step, not part of Persist's success definition).
func (p *Persister) persistMediaFile(ctx context.Context, item *domain.Item, uf *domain.UnmatchedFile) error {
	if mf, err := p.mediaFiles.GetByHash(ctx, uf.OSHash, uf.SHA1, uf.MD5, uf.SHA512); err == nil {
		return p.ensureItemImported(ctx, item, mf.ItemID)
	} else if !errors.Is(err, ports.ErrNotFound) {
		return err
	}

	mf := &domain.MediaFile{
		ID: domain.NewID(), ItemID: item.ID, Path: uf.Path, Size: uf.Size,
		OSHash: uf.OSHash, MD5: uf.MD5, SHA1: uf.SHA1, SHA512: uf.SHA512,
	}
	if err := mf.Validate(); err != nil {
		return err
	}
	if err := p.mediaFiles.Create(ctx, mf); err != nil {
		return err
	}

	item.Status = domain.ItemStatusImported
	if err := p.items.Update(ctx, item); err != nil {
		return err
	}

	if _, err := p.organizer.Organize(ctx, mf.ID); err != nil {
		p.logger.ErrorContext(ctx, "auto-organize failed", "media_file.id", mf.ID, "error", err)
	}
	return nil
}

// ensureItemImported flips item's Status to imported if it isn't already —
// used when a MediaFile matching uf's hashes already exists (a prior
// attempt got this far). If that MediaFile belongs to a different Item than
// the one this call resolved to, that's an unexpected identity mismatch
// (e.g. the same file previously imported under a different Studio) —
// logged, not auto-repaired.
func (p *Persister) ensureItemImported(ctx context.Context, item *domain.Item, mediaFileItemID string) error {
	if mediaFileItemID != item.ID {
		p.logger.WarnContext(ctx, "media file already linked to a different item", "item.id", item.ID, "media_file.item_id", mediaFileItemID)
		return nil
	}
	if item.Status == domain.ItemStatusImported {
		return nil
	}
	item.Status = domain.ItemStatusImported
	return p.items.Update(ctx, item)
}

// persistPerformers unions every resolved candidate's performer credits
// onto itemID — see this file's package comment. No cross-provider
// person-identity resolution is attempted (a StashDB-sourced and a
// ThePornDB-sourced credit for the same real performer become two distinct
// Person rows); documented as unresolved in
// docs/technical/afterdark-data_model.md's Performer section, not an
// oversight here.
func (p *Persister) persistPerformers(ctx context.Context, itemID string, resolved []resolvedCandidate) error {
	for _, rc := range resolved {
		for _, rp := range rc.performers {
			personID, err := p.getOrCreatePerson(ctx, rp)
			if err != nil {
				return fmt.Errorf("resolving performer %q: %w", rp.name, err)
			}
			if err := p.attachCredit(ctx, itemID, personID, rp.creditedAs); err != nil {
				return fmt.Errorf("crediting performer %q: %w", rp.name, err)
			}
		}
	}
	return nil
}

// getOrCreatePerson resolves rp to a Person.ID via ExternalID(person,
// source, externalRef) — see resolvedPerformer's doc comment for why a
// ThePornDB alias performer's own ID is folded into externalRef's
// canonical value rather than linked as a second row. Ensures a
// PerformerProfile exists for the resolved Person.
func (p *Persister) getOrCreatePerson(ctx context.Context, rp resolvedPerformer) (string, error) {
	personID, err := p.resolveOrCreate(ctx, domain.EntityTypePerson, externalIDSourceFor(rp.source), rp.externalRef, func(ctx context.Context) (string, error) {
		person := &domain.Person{
			ID:          domain.NewID(),
			Name:        rp.name,
			Aliases:     rp.aliases,
			Gender:      rp.gender,
			BirthDate:   rp.birthDate,
			Nationality: rp.nationality,
			Overview:    rp.overview,
			Monitored:   true,
			MonitorMode: domain.MonitorModeAll,
		}
		if err := person.Validate(); err != nil {
			return "", err
		}
		if err := p.persons.Create(ctx, person); err != nil {
			return "", err
		}
		return person.ID, nil
	}, p.persons.Delete)
	if err != nil {
		return "", err
	}

	if err := p.ensurePerformerProfile(ctx, personID, rp); err != nil {
		return "", err
	}
	return personID, nil
}

// ensurePerformerProfile creates rp's PerformerProfile the first time
// personID is resolved — not re-synced on a later Persist call against the
// same Person (same "create once, don't re-sync scalars on retry"
// simplification reconcileSceneScalars documents for the scene itself, kept
// consistent here).
func (p *Persister) ensurePerformerProfile(ctx context.Context, personID string, rp resolvedPerformer) error {
	if _, err := p.performerProfiles.Get(ctx, personID); err == nil {
		return nil
	} else if !errors.Is(err, ports.ErrNotFound) {
		return err
	}

	profile := &afterdark.PerformerProfile{
		PersonID:        personID,
		CupSize:         rp.cupSize,
		BandSize:        rp.bandSize,
		BreastType:      rp.breastType,
		CareerStartYear: rp.careerStartYear,
		CareerEndYear:   rp.careerEndYear,
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	return p.performerProfiles.Create(ctx, profile)
}

// attachCredit creates the ItemPerson(itemID, personID, performerRole)
// credit if it doesn't already exist — a plain existence check, not
// ADR-0026's reservation mechanism (ItemPerson's identity is its own
// composite key, no separate uniqueness race to protect), same "guarded by
// a plain existence check" treatment music.Persister's cover-art step uses.
func (p *Persister) attachCredit(ctx context.Context, itemID, personID, creditedAs string) error {
	if _, err := p.itemPeople.Get(ctx, itemID, personID, performerRole); err == nil {
		return nil
	} else if !errors.Is(err, ports.ErrNotFound) {
		return err
	}

	ip := &domain.ItemPerson{ItemID: itemID, PersonID: personID, Role: performerRole, CreditedAs: creditedAs}
	if err := ip.Validate(); err != nil {
		return err
	}
	return p.itemPeople.Create(ctx, ip)
}

// persistTags unions every resolved candidate's scene tags onto itemID,
// deduplicated case-insensitively across the whole set.
func (p *Persister) persistTags(ctx context.Context, itemID string, resolved []resolvedCandidate) error {
	seen := map[string]bool{}
	for _, rc := range resolved {
		for _, name := range rc.tags {
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := p.attachTag(ctx, itemID, name); err != nil {
				return fmt.Errorf("attaching tag %q: %w", name, err)
			}
		}
	}
	return nil
}

// attachTag get-or-creates Tag{Key: tagKey, Value: name} (ADR-0019's
// existing get-or-create Create contract) and attaches it to itemID if not
// already assigned — same plain-existence-check dedup guard as
// attachCredit.
func (p *Persister) attachTag(ctx context.Context, itemID, name string) error {
	tag := &domain.Tag{ID: domain.NewID(), Key: tagKey, Value: name, Scope: domain.TagScopeMetadata}
	if err := tag.Validate(); err != nil {
		return err
	}
	if err := p.tags.Create(ctx, tag); err != nil {
		return err
	}

	if _, err := p.tagAssignments.Get(ctx, tag.ID, domain.EntityTypeItem, itemID); err == nil {
		return nil
	} else if !errors.Is(err, ports.ErrNotFound) {
		return err
	}

	ta := &domain.TagAssignment{TagID: tag.ID, EntityType: domain.EntityTypeItem, EntityID: itemID}
	if err := ta.Validate(); err != nil {
		return err
	}
	return p.tagAssignments.Create(ctx, ta)
}

// persistSceneImages implements AD9 (issue #563): unions every resolved
// candidate's own poster/screenshot image URLs onto item, guarded by the
// same coarse existence check music.Persister's persistCoverArt uses for
// cover art — any image already attached to this scene means a prior
// Persist got this far, so the whole step is skipped rather than
// re-fetching and re-diffing individual URLs. Dedup is by exact URL across
// candidates, order preserved (candidate order, then each candidate's own
// image order) and recorded as Image.Priority, the same "lower is shown
// first" convention persistCoverArt's rank encodes. A single image's fetch
// or store failure is logged and skipped, not fatal to Persist — images
// are an enhancement on top of a scene's core identity, same tolerance
// this file already gives a per-candidate resolve failure or the
// Organizer's own best-effort call.
func (p *Persister) persistSceneImages(ctx context.Context, item *domain.Item, resolved []resolvedCandidate) error {
	existing, _, err := p.images.List(ctx, sceneOwnerType, item.ID, 1, "")
	if err != nil {
		return fmt.Errorf("checking existing images for scene %s: %w", item.ID, err)
	}
	if len(existing) > 0 {
		return nil
	}

	seen := map[string]bool{}
	priority := 0
	for _, rc := range resolved {
		for _, img := range rc.images {
			if img.url == "" || seen[img.url] {
				continue
			}
			seen[img.url] = true
			p.attachSceneImage(ctx, item.ID, rc.source, img, priority)
			priority++
		}
	}
	return nil
}

// attachSceneImage fetches img's bytes via ImageFetcher, writes them via
// ImageStore (keyed on the Image's own generated ID, per
// docs/adr/0013-image-blob-storage.md's sharding decision), then persists
// the Image metadata row — bytes before the row that points at them, per
// that ADR's ordering requirement. Errors are logged and swallowed, never
// returned — see persistSceneImages' doc comment.
func (p *Persister) attachSceneImage(ctx context.Context, itemID, source string, img resolvedImage, priority int) {
	rc, err := p.imageFetcher.Fetch(ctx, img.url)
	if err != nil {
		p.logger.WarnContext(ctx, "fetching scene image failed, skipping", "item.id", itemID, "url", img.url, "error", err)
		return
	}
	defer func() { _ = rc.Close() }()

	domainImg := &domain.Image{
		ID:        domain.NewID(),
		OwnerType: sceneOwnerType,
		OwnerID:   itemID,
		ImageType: domain.ImageTypePoster,
		Width:     img.width,
		Height:    img.height,
		Priority:  priority,
		Source:    source,
	}

	key, err := p.imageStore.Put(ctx, sceneOwnerType, domainImg.ID, rc)
	if err != nil {
		p.logger.WarnContext(ctx, "writing scene image failed, skipping", "item.id", itemID, "url", img.url, "error", err)
		return
	}
	domainImg.URL = key

	if err := domainImg.Validate(); err != nil {
		p.logger.WarnContext(ctx, "scene image failed validation, skipping", "item.id", itemID, "url", img.url, "error", err)
		return
	}
	if err := p.images.Create(ctx, domainImg); err != nil {
		p.logger.WarnContext(ctx, "creating scene image row failed", "item.id", itemID, "url", img.url, "error", err)
	}
}

// parseSceneDate parses a provider scene date ("2006-01-02", the shape both
// StashDB's release_date and ThePornDB's date use), returning nil for an
// empty or unparseable string — a scene's Date is optional.
func parseSceneDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t
	}
	return nil
}

// containsFold reports whether ss contains s, case-insensitively.
func containsFold(ss []string, s string) bool {
	for _, x := range ss {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// resolveStashDBScene normalizes a StashDB ports.Scene (already the full
// detail — Scene.Performers[].Performer carries every field this cascade
// needs, no further per-performer lookup required) into a resolvedCandidate.
func resolveStashDBScene(scene ports.Scene, c domain.MatchCandidate) resolvedCandidate {
	rc := resolvedCandidate{
		source:      sourceStashDB,
		externalRef: scene.ID,
		title:       scene.Title,
		overview:    scene.Details,
		date:        parseSceneDate(scene.ReleaseDate),
	}
	if scene.Studio != nil {
		rc.studio = &resolvedStudio{source: sourceStashDB, externalRef: scene.Studio.ID, name: scene.Studio.Name}
	}
	for _, t := range scene.Tags {
		rc.tags = append(rc.tags, t.Name)
	}
	for _, pa := range scene.Performers {
		rc.performers = append(rc.performers, resolveStashDBPerformer(pa))
	}
	for _, img := range scene.Images {
		rc.images = append(rc.images, resolvedImage{url: img.URL, width: img.Width, height: img.Height})
	}
	if code, ok := c.Metadata["jav_code"].(string); ok && code != "" {
		rc.javCode = code
		rc.isJAV = true
	}
	return rc
}

// resolveStashDBPerformer normalizes one StashDB PerformerAppearance.
// StashDB's GenderEnum values are exactly domain.Gender's own constants
// (per person.go's doc comment), so mapping is a direct uppercase-and-
// validate, no lookup table needed. StashDB's Performer DTO carries no
// Tattoos/Piercings/Bio-equivalent fields (AD1 never fetched them) — left
// unset, not guessed.
func resolveStashDBPerformer(pa ports.PerformerAppearance) resolvedPerformer {
	perf := pa.Performer
	creditedAs := ""
	if pa.As != "" && pa.As != perf.Name {
		creditedAs = pa.As
	}

	gender := domain.Gender(strings.ToUpper(strings.TrimSpace(perf.Gender)))
	if !gender.Valid() {
		gender = domain.GenderUnknown
	}

	var birthDate *time.Time
	if perf.BirthDate != "" {
		if t, err := time.Parse("2006-01-02", perf.BirthDate); err == nil {
			birthDate = &t
		}
	}

	var bandSize string
	if perf.BandSize > 0 {
		bandSize = strconv.Itoa(perf.BandSize)
	}

	return resolvedPerformer{
		source:          sourceStashDB,
		externalRef:     perf.ID,
		name:            perf.Name,
		creditedAs:      creditedAs,
		aliases:         append([]string{}, perf.Aliases...),
		gender:          gender,
		birthDate:       birthDate,
		nationality:     perf.Country,
		cupSize:         perf.CupSize,
		bandSize:        bandSize,
		breastType:      perf.BreastType,
		careerStartYear: perf.CareerStartYear,
		careerEndYear:   perf.CareerEndYear,
	}
}

// resolveTPDBScene normalizes a ThePornDB ports.TPDBScene into a
// resolvedCandidate. isJAV/javCode prefer the JAV-code tier's own parsed
// Metadata["jav_code"] (identifier.go's javCodeTier) when present, falling
// back to the resolved scene's own SKU whenever ThePornDB itself reports
// Type=="JAV" — covering a JAV scene resolved via a different tier
// (fingerprint/direct-id/fuzzy), where identifier.go never ran the JAV-code
// parser at all.
func resolveTPDBScene(scene ports.TPDBScene, c domain.MatchCandidate) resolvedCandidate {
	rc := resolvedCandidate{
		source:      sourceTPDB,
		externalRef: scene.ID,
		title:       scene.Title,
		overview:    scene.Description,
		date:        parseSceneDate(scene.Date),
		isJAV:       strings.EqualFold(scene.Type, "JAV"),
	}
	if scene.Site != nil {
		rc.studio = &resolvedStudio{source: sourceTPDB, externalRef: scene.Site.UUID, name: scene.Site.Name}
	}
	for _, t := range scene.Tags {
		rc.tags = append(rc.tags, t.Name)
	}
	for _, perf := range scene.Performers {
		rc.performers = append(rc.performers, resolveTPDBPerformer(perf))
	}
	rc.images = tpdbSceneImages(scene)

	if code, ok := c.Metadata["jav_code"].(string); ok && code != "" {
		rc.javCode = code
	} else if rc.isJAV && scene.SKU != "" {
		rc.javCode = scene.SKU
	}
	return rc
}

// tpdbSceneImages collects scene's own image URLs, deduplicated: Image
// (the scene's still/screenshot) and Posters.Full (the largest of the four
// pre-cropped poster sizes ports.TPDBPosters carries — Large/Medium/Small
// are resized copies of that same picture, not distinct images, so only
// the largest is fetched), falling back to the bare Poster field when
// Posters.Full is empty. ThePornDB's plain URL strings carry no
// width/height the way StashDB's Image DTO does.
func tpdbSceneImages(scene ports.TPDBScene) []resolvedImage {
	poster := scene.Posters.Full
	if poster == "" {
		poster = scene.Poster
	}

	var images []resolvedImage
	if scene.Image != "" {
		images = append(images, resolvedImage{url: scene.Image})
	}
	if poster != "" && poster != scene.Image {
		images = append(images, resolvedImage{url: poster})
	}
	return images
}

// resolveTPDBPerformer normalizes one ThePornDB TPDBPerformer. When perf is
// a site-specific alias record (Parent != nil, per the port's own doc
// comment), the canonical Person is built from the parent — externalRef is
// always the canonical parent's own ID, never the alias record's, see
// resolvedPerformer's doc comment for why — and the alias's display name
// becomes both this credit's CreditedAs and an entry in the canonical
// Person's Aliases, per docs/technical/afterdark-data_model.md §5 decision
// 5. BirthDate uses Extras.BirthdayTimestamp (a unix timestamp) rather than
// parsing Extras.Birthday's free-text string, which has no confirmed fixed
// format.
func resolveTPDBPerformer(perf ports.TPDBPerformer) resolvedPerformer {
	canonical := perf
	creditedAs := ""
	if perf.Parent != nil {
		canonical = *perf.Parent
		if perf.Name != canonical.Name {
			creditedAs = perf.Name
		}
	}

	var birthDate *time.Time
	if canonical.Extras.BirthdayTimestamp > 0 {
		t := time.Unix(canonical.Extras.BirthdayTimestamp, 0).UTC()
		birthDate = &t
	}

	aliases := append([]string{}, canonical.Aliases...)
	if creditedAs != "" && !containsFold(aliases, creditedAs) {
		aliases = append(aliases, creditedAs)
	}

	return resolvedPerformer{
		source:          sourceTPDB,
		externalRef:     canonical.ID,
		name:            canonical.Name,
		creditedAs:      creditedAs,
		aliases:         aliases,
		gender:          mapTPDBGender(canonical.Extras.Gender),
		birthDate:       birthDate,
		nationality:     canonical.Extras.Nationality,
		overview:        canonical.Bio,
		cupSize:         canonical.Extras.CupSize,
		careerStartYear: canonical.Extras.CareerStartYear,
		careerEndYear:   canonical.Extras.CareerEndYear,
	}
}

// mapTPDBGender maps ThePornDB's free-text Extras.Gender (closer to free
// text than StashDB's typed enum, per
// docs/technical/afterdark-data_model.md §3) onto domain.Gender,
// case-insensitively. An unrecognized or empty value maps to
// domain.GenderUnknown, never a guess.
func mapTPDBGender(s string) domain.Gender {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "MALE":
		return domain.GenderMale
	case "FEMALE":
		return domain.GenderFemale
	case "TRANSGENDER MALE", "TRANSGENDER_MALE", "TRANS MALE":
		return domain.GenderTransgenderMale
	case "TRANSGENDER FEMALE", "TRANSGENDER_FEMALE", "TRANS FEMALE":
		return domain.GenderTransgenderFemale
	case "INTERSEX":
		return domain.GenderIntersex
	case "NON-BINARY", "NON_BINARY", "NONBINARY":
		return domain.GenderNonBinary
	default:
		return domain.GenderUnknown
	}
}
