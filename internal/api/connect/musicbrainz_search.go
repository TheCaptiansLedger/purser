package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// musicBrainzSearchService is the narrow interface MusicBrainzSearchHandler
// depends on — see personService for the DIP convention this follows.
type musicBrainzSearchService interface {
	SearchReleaseGroups(ctx context.Context, artistName, albumName string) ([]ports.ReleaseGroup, error)
	ListReleasesForReleaseGroup(ctx context.Context, releaseGroupMBID string) ([]ports.Release, error)
	SearchArtists(ctx context.Context, query string) ([]ports.Artist, error)
	ListReleaseGroupsForArtist(ctx context.Context, artistMBID string) ([]ports.ReleaseGroup, error)
	GetArtist(ctx context.Context, mbid string) (*ports.Artist, error)
	GetRelease(ctx context.Context, mbid string) (*ports.Release, error)
}

// MusicBrainzSearchHandler implements musicv1connect.MusicBrainzServiceHandler
// — see proto/purser/music/v1/musicbrainz_search.proto's own doc comment
// for why this exists.
type MusicBrainzSearchHandler struct {
	musicv1connect.UnimplementedMusicBrainzServiceHandler
	svc    musicBrainzSearchService
	logger *slog.Logger
}

// NewMusicBrainzSearchHandler constructs a MusicBrainzSearchHandler backed
// by svc.
func NewMusicBrainzSearchHandler(svc musicBrainzSearchService, logger *slog.Logger) *MusicBrainzSearchHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MusicBrainzSearchHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "MusicBrainzService")}
}

// SearchReleaseGroups implements musicv1connect.MusicBrainzServiceHandler.
func (h *MusicBrainzSearchHandler) SearchReleaseGroups(ctx context.Context, req *connect.Request[musicv1.SearchMusicBrainzReleaseGroupsRequest]) (*connect.Response[musicv1.SearchMusicBrainzReleaseGroupsResponse], error) {
	rgs, err := h.svc.SearchReleaseGroups(ctx, req.Msg.GetArtistName(), req.Msg.GetAlbumName())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*musicv1.MusicBrainzReleaseGroup, len(rgs))
	for i, rg := range rgs {
		out[i] = releaseGroupToProto(rg)
	}
	return connect.NewResponse(&musicv1.SearchMusicBrainzReleaseGroupsResponse{ReleaseGroups: out}), nil
}

// ListReleasesForReleaseGroup implements
// musicv1connect.MusicBrainzServiceHandler.
func (h *MusicBrainzSearchHandler) ListReleasesForReleaseGroup(ctx context.Context, req *connect.Request[musicv1.ListMusicBrainzReleasesRequest]) (*connect.Response[musicv1.ListMusicBrainzReleasesResponse], error) {
	releases, err := h.svc.ListReleasesForReleaseGroup(ctx, req.Msg.GetReleaseGroupMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*musicv1.MusicBrainzRelease, len(releases))
	for i, r := range releases {
		out[i] = releaseToProto(r)
	}
	return connect.NewResponse(&musicv1.ListMusicBrainzReleasesResponse{Releases: out}), nil
}

// SearchArtists implements musicv1connect.MusicBrainzServiceHandler.
func (h *MusicBrainzSearchHandler) SearchArtists(ctx context.Context, req *connect.Request[musicv1.SearchMusicBrainzArtistsRequest]) (*connect.Response[musicv1.SearchMusicBrainzArtistsResponse], error) {
	artists, err := h.svc.SearchArtists(ctx, req.Msg.GetQuery())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*musicv1.MusicBrainzArtist, len(artists))
	for i, a := range artists {
		out[i] = artistToProto(a)
	}
	return connect.NewResponse(&musicv1.SearchMusicBrainzArtistsResponse{Artists: out}), nil
}

// ListReleaseGroupsForArtist implements
// musicv1connect.MusicBrainzServiceHandler.
func (h *MusicBrainzSearchHandler) ListReleaseGroupsForArtist(ctx context.Context, req *connect.Request[musicv1.ListMusicBrainzArtistReleaseGroupsRequest]) (*connect.Response[musicv1.ListMusicBrainzArtistReleaseGroupsResponse], error) {
	rgs, err := h.svc.ListReleaseGroupsForArtist(ctx, req.Msg.GetArtistMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*musicv1.MusicBrainzReleaseGroup, len(rgs))
	for i, rg := range rgs {
		out[i] = releaseGroupToProto(rg)
	}
	return connect.NewResponse(&musicv1.ListMusicBrainzArtistReleaseGroupsResponse{ReleaseGroups: out}), nil
}

// GetArtist implements musicv1connect.MusicBrainzServiceHandler.
func (h *MusicBrainzSearchHandler) GetArtist(ctx context.Context, req *connect.Request[musicv1.GetMusicBrainzArtistRequest]) (*connect.Response[musicv1.GetMusicBrainzArtistResponse], error) {
	a, err := h.svc.GetArtist(ctx, req.Msg.GetMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	officialURL, wikipediaURL := artistLinksFromRelations(a.Relations)
	return connect.NewResponse(&musicv1.GetMusicBrainzArtistResponse{
		Artist:       artistToProto(*a),
		Isnis:        a.ISNIs,
		OfficialUrl:  officialURL,
		WikipediaUrl: wikipediaURL,
		Members:      membersFromRelations(a.Relations),
	}), nil
}

// GetRelease implements musicv1connect.MusicBrainzServiceHandler — the one
// RPC in this service that returns a release's full track listing (see
// GetMusicBrainzReleaseResponse's own doc comment). Read-only passthrough
// per ADR 0027: no ranking, no persistence.
func (h *MusicBrainzSearchHandler) GetRelease(ctx context.Context, req *connect.Request[musicv1.GetMusicBrainzReleaseRequest]) (*connect.Response[musicv1.GetMusicBrainzReleaseResponse], error) {
	r, err := h.svc.GetRelease(ctx, req.Msg.GetMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	media := make([]*musicv1.MusicBrainzMedium, len(r.Media))
	for i, m := range r.Media {
		media[i] = mediumToProto(m)
	}
	return connect.NewResponse(&musicv1.GetMusicBrainzReleaseResponse{
		Release: releaseToProto(*r),
		Media:   media,
	}), nil
}

// mediumToProto maps ports.Medium (and its own Tracks) to its wire shape.
func mediumToProto(m ports.Medium) *musicv1.MusicBrainzMedium {
	tracks := make([]*musicv1.MusicBrainzTrack, len(m.Tracks))
	for i, t := range m.Tracks {
		var recordingMBID string
		if t.Recording != nil {
			recordingMBID = t.Recording.ID
		}
		tracks[i] = &musicv1.MusicBrainzTrack{
			Position:      int32(t.Position), //nolint:gosec // a track's position on a medium is never remotely close to overflowing int32
			Number:        t.Number,
			Title:         t.Title,
			LengthMs:      int32(t.Length), //nolint:gosec // a track's length in ms is never remotely close to overflowing int32
			RecordingMbid: recordingMBID,
		}
	}
	return &musicv1.MusicBrainzMedium{
		Position: int32(m.Position), //nolint:gosec // a medium's position on a release is never remotely close to overflowing int32
		Format:   m.Format,
		Tracks:   tracks,
	}
}

// artistLinksFromRelations pulls the official-homepage and Wikipedia URLs
// out of an Artist's Relations — MusicBrainz's own relation-type
// vocabulary ("official homepage", "wikipedia"), read-only passthrough per
// ADR 0027. Either or both may be absent; callers treat an empty string as
// "not offered by MusicBrainz for this artist," not an error.
func artistLinksFromRelations(relations []ports.Relation) (officialURL, wikipediaURL string) {
	for _, r := range relations {
		if r.URL == nil {
			continue
		}
		switch r.Type {
		case "official homepage":
			officialURL = r.URL.Resource
		case "wikipedia":
			wikipediaURL = r.URL.Resource
		}
	}
	return officialURL, wikipediaURL
}

// membersFromRelations pulls band-member edges ("member of band",
// Relation.Artist populated) out of an Artist's Relations and maps them to
// the wire shape — the Add Artist flow's source for creating the
// corresponding Person/EntryPerson rows for a Group artist.
func membersFromRelations(relations []ports.Relation) []*musicv1.MusicBrainzArtistMember {
	members := make([]*musicv1.MusicBrainzArtistMember, 0)
	for _, r := range relations {
		if r.Type != "member of band" || r.Artist == nil {
			continue
		}
		members = append(members, &musicv1.MusicBrainzArtistMember{
			Mbid:       r.Artist.ID,
			Name:       r.Artist.Name,
			Attributes: r.Attributes,
			Begin:      r.Begin,
			End:        r.End,
			Ended:      r.Ended,
		})
	}
	return members
}

// artistToProto maps ports.Artist (internal/adapters/musicbrainz's own DTO)
// to its wire shape — the browse-list summary, per
// MusicBrainzArtist's own doc comment.
func artistToProto(a ports.Artist) *musicv1.MusicBrainzArtist {
	aliases := make([]string, 0, len(a.Aliases))
	for _, alias := range a.Aliases {
		if alias.Name != "" {
			aliases = append(aliases, alias.Name)
		}
	}
	return &musicv1.MusicBrainzArtist{
		Mbid:           a.ID,
		Name:           a.Name,
		SortName:       a.SortName,
		Disambiguation: a.Disambiguation,
		Type:           a.Type,
		Country:        a.Country,
		LifeSpanBegin:  a.LifeSpan.Begin,
		LifeSpanEnd:    a.LifeSpan.End,
		Aliases:        aliases,
	}
}

// releaseGroupToProto maps ports.ReleaseGroup (internal/adapters/musicbrainz's
// own DTO) to its wire shape.
func releaseGroupToProto(rg ports.ReleaseGroup) *musicv1.MusicBrainzReleaseGroup {
	return &musicv1.MusicBrainzReleaseGroup{
		Mbid:             rg.ID,
		Title:            rg.Title,
		Disambiguation:   rg.Disambiguation,
		PrimaryType:      rg.PrimaryType,
		SecondaryTypes:   rg.SecondaryTypes,
		FirstReleaseDate: rg.FirstReleaseDate,
	}
}

// releaseToProto maps ports.Release to its wire shape, summarizing the
// first medium/label-info entry per the proto's own doc comment — a
// caller wanting the full tracklist accepts this release's Mbid via
// AcceptCandidateRequest instead of re-deriving it from this summary.
func releaseToProto(r ports.Release) *musicv1.MusicBrainzRelease {
	// TrackCount (a medium's own scalar field) is used here, not
	// len(m.Tracks) — ListReleasesForReleaseGroup's inc= never requests
	// "recordings", so Tracks is always empty for every result this
	// method returns (confirmed live against the real API); TrackCount
	// itself is populated by "media" alone (see identifier.go's own
	// releaseTrackCount, which relies on the same fact).
	var format string
	mediumCount := len(r.Media)
	trackCount := 0
	for _, m := range r.Media {
		trackCount += m.TrackCount
	}
	if mediumCount > 0 {
		format = r.Media[0].Format
	}

	var label, catalogNumber string
	if len(r.LabelInfo) > 0 {
		label = r.LabelInfo[0].Label.Name
		catalogNumber = r.LabelInfo[0].CatalogNumber
	}

	names := make([]string, 0, len(r.ArtistCredit))
	for _, ac := range r.ArtistCredit {
		if ac.Name != "" {
			names = append(names, ac.Name)
		}
	}

	return &musicv1.MusicBrainzRelease{
		Mbid:              r.ID,
		Title:             r.Title,
		Disambiguation:    r.Disambiguation,
		Country:           r.Country,
		Date:              r.Date,
		Barcode:           r.Barcode,
		Status:            r.Status,
		Format:            format,
		MediumCount:       int32(mediumCount), //nolint:gosec // a MusicBrainz release's medium count is never remotely close to overflowing int32
		TrackCount:        int32(trackCount),  //nolint:gosec // same — a release's total track count
		ArtistCreditNames: names,
		Label:             label,
		CatalogNumber:     catalogNumber,
	}
}
