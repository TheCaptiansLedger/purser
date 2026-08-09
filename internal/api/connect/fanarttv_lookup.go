package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// fanartTVLookupService is the narrow interface FanartTVHandler depends
// on — see personService for the DIP convention this follows.
type fanartTVLookupService interface {
	LookupArtist(ctx context.Context, mbid string) (*ports.FanartArtist, error)
}

// FanartTVHandler implements musicv1connect.FanartTVServiceHandler — see
// proto/purser/music/v1/fanarttv.proto's own doc comment for why this
// exists.
type FanartTVHandler struct {
	musicv1connect.UnimplementedFanartTVServiceHandler
	svc    fanartTVLookupService
	logger *slog.Logger
}

// NewFanartTVHandler constructs a FanartTVHandler backed by svc.
func NewFanartTVHandler(svc fanartTVLookupService, logger *slog.Logger) *FanartTVHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &FanartTVHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "FanartTVService")}
}

// LookupArtist implements musicv1connect.FanartTVServiceHandler.
func (h *FanartTVHandler) LookupArtist(ctx context.Context, req *connect.Request[musicv1.LookupFanartTVArtistRequest]) (*connect.Response[musicv1.LookupFanartTVArtistResponse], error) {
	a, err := h.svc.LookupArtist(ctx, req.Msg.GetMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.LookupFanartTVArtistResponse{Artist: fanartArtistToProto(*a)}), nil
}

// fanartArtistToProto maps ports.FanartArtist (internal/adapters/fanarttv's
// own DTO) to its wire shape.
func fanartArtistToProto(a ports.FanartArtist) *musicv1.FanartTVArtist {
	albums := make(map[string]*musicv1.FanartTVAlbumImages, len(a.Albums))
	for mbid, images := range a.Albums {
		albums[mbid] = fanartAlbumImagesToProto(images)
	}
	return &musicv1.FanartTVArtist{
		Name:                a.Name,
		Mbid:                a.MBID,
		ArtistThumb:         fanartImagesToProto(a.ArtistThumb),
		ArtistBackground:    fanartImagesToProto(a.ArtistBackground),
		Artist_4KBackground: fanartImagesToProto(a.Artist4KBackground),
		HdMusicLogo:         fanartImagesToProto(a.HDMusicLogo),
		MusicLogo:           fanartImagesToProto(a.MusicLogo),
		MusicBanner:         fanartImagesToProto(a.MusicBanner),
		Albums:              albums,
	}
}

// fanartAlbumImagesToProto maps ports.FanartAlbumImages to its wire shape.
func fanartAlbumImagesToProto(images ports.FanartAlbumImages) *musicv1.FanartTVAlbumImages {
	cdArt := make([]*musicv1.FanartTVCDArt, len(images.CDArt))
	for i, c := range images.CDArt {
		cdArt[i] = &musicv1.FanartTVCDArt{
			Id:    c.ID,
			Url:   c.URL,
			Likes: c.Likes,
			Disc:  c.Disc,
			Size:  c.Size,
		}
	}
	return &musicv1.FanartTVAlbumImages{
		AlbumCover: fanartImagesToProto(images.AlbumCover),
		CdArt:      cdArt,
	}
}

// fanartImagesToProto maps a []ports.FanartImage slice to its wire shape.
func fanartImagesToProto(images []ports.FanartImage) []*musicv1.FanartTVImage {
	out := make([]*musicv1.FanartTVImage, len(images))
	for i, img := range images {
		out[i] = &musicv1.FanartTVImage{
			Id:    img.ID,
			Url:   img.URL,
			Likes: img.Likes,
			Lang:  img.Lang,
		}
	}
	return out
}
