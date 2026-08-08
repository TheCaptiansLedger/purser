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

	var label string
	if len(r.LabelInfo) > 0 {
		label = r.LabelInfo[0].Label.Name
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
	}
}
