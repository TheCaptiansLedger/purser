package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// theAudioDBLookupService is the narrow interface TheAudioDBHandler depends
// on — see personService for the DIP convention this follows.
type theAudioDBLookupService interface {
	LookupArtist(ctx context.Context, mbid string) (*ports.TADBArtist, error)
	LookupAlbum(ctx context.Context, releaseGroupMBID string) (*ports.TADBAlbum, error)
}

// TheAudioDBHandler implements musicv1connect.TheAudioDBServiceHandler —
// see proto/purser/music/v1/theaudiodb.proto's own doc comment for why
// this exists.
type TheAudioDBHandler struct {
	musicv1connect.UnimplementedTheAudioDBServiceHandler
	svc    theAudioDBLookupService
	logger *slog.Logger
}

// NewTheAudioDBHandler constructs a TheAudioDBHandler backed by svc.
func NewTheAudioDBHandler(svc theAudioDBLookupService, logger *slog.Logger) *TheAudioDBHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TheAudioDBHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "TheAudioDBService")}
}

// LookupArtist implements musicv1connect.TheAudioDBServiceHandler.
func (h *TheAudioDBHandler) LookupArtist(ctx context.Context, req *connect.Request[musicv1.LookupTheAudioDBArtistRequest]) (*connect.Response[musicv1.LookupTheAudioDBArtistResponse], error) {
	a, err := h.svc.LookupArtist(ctx, req.Msg.GetMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.LookupTheAudioDBArtistResponse{Artist: tadbArtistToProto(*a)}), nil
}

// LookupAlbum implements musicv1connect.TheAudioDBServiceHandler.
func (h *TheAudioDBHandler) LookupAlbum(ctx context.Context, req *connect.Request[musicv1.LookupTheAudioDBAlbumRequest]) (*connect.Response[musicv1.LookupTheAudioDBAlbumResponse], error) {
	a, err := h.svc.LookupAlbum(ctx, req.Msg.GetReleaseGroupMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.LookupTheAudioDBAlbumResponse{Album: tadbAlbumToProto(*a)}), nil
}

// tadbArtistToProto maps ports.TADBArtist (internal/adapters/theaudiodb's
// own DTO) to its wire shape.
func tadbArtistToProto(a ports.TADBArtist) *musicv1.TheAudioDBArtist {
	return &musicv1.TheAudioDBArtist{
		Mbid:          a.MusicBrainzID,
		Name:          a.Name,
		AlternateName: a.AlternateName,
		Label:         a.Label,
		FormedYear:    a.FormedYear,
		BornYear:      a.BornYear,
		DiedYear:      a.DiedYear,
		Disbanded:     a.Disbanded,
		Style:         a.Style,
		Genre:         a.Genre,
		Mood:          a.Mood,
		Gender:        a.Gender,
		Country:       a.Country,
		CountryCode:   a.CountryCode,
		IsniCode:      a.ISNICode,
		Members:       a.Members,
		Followers:     a.Followers,
		Popularity:    a.Popularity,
		Charted:       a.Charted,
		Website:       a.Website,
		Facebook:      a.Facebook,
		Twitter:       a.Twitter,
		Biography:     a.Biography,
		Locked:        a.Locked,
		Thumb:         a.Thumb,
		Logo:          a.Logo,
		Cutout:        a.Cutout,
		Clearart:      a.Clearart,
		WideThumb:     a.WideThumb,
		Fanart:        a.Fanart,
		Fanart2:       a.Fanart2,
		Fanart3:       a.Fanart3,
		Fanart4:       a.Fanart4,
		Banner:        a.Banner,
	}
}

// tadbAlbumToProto maps ports.TADBAlbum to its wire shape.
func tadbAlbumToProto(a ports.TADBAlbum) *musicv1.TheAudioDBAlbum {
	return &musicv1.TheAudioDBAlbum{
		Mbid:          a.MusicBrainzID,
		ArtistMbid:    a.MusicBrainzArtistID,
		Title:         a.Title,
		ArtistName:    a.ArtistName,
		YearReleased:  a.YearReleased,
		Style:         a.Style,
		Genre:         a.Genre,
		Label:         a.Label,
		ReleaseFormat: a.ReleaseFormat,
		Description:   a.Description,
		Mood:          a.Mood,
		Locked:        a.Locked,
		Thumb:         a.Thumb,
		ThumbHq:       a.ThumbHQ,
		Back:          a.Back,
		CdArt:         a.CDArt,
		Spine:         a.Spine,
		ThreeDCase:    a.ThreeDCase,
		ThreeDFlat:    a.ThreeDFlat,
		ThreeDFace:    a.ThreeDFace,
		ThreeDThumb:   a.ThreeDThumb,
	}
}
