package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// stashDBLookupService is the narrow interface StashDBHandler depends
// on — see personService for the DIP convention this follows.
type stashDBLookupService interface {
	LookupPerformer(ctx context.Context, id string) (*ports.Performer, error)
	SearchPerformers(ctx context.Context, term string) ([]ports.Performer, error)
}

// StashDBHandler implements afterdarkv1connect.StashDBServiceHandler — see
// proto/purser/afterdark/v1/stashdb.proto's own doc comment for why this
// exists.
type StashDBHandler struct {
	afterdarkv1connect.UnimplementedStashDBServiceHandler
	svc    stashDBLookupService
	logger *slog.Logger
}

// NewStashDBHandler constructs a StashDBHandler backed by svc.
func NewStashDBHandler(svc stashDBLookupService, logger *slog.Logger) *StashDBHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &StashDBHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "StashDBService")}
}

// LookupPerformer implements afterdarkv1connect.StashDBServiceHandler.
func (h *StashDBHandler) LookupPerformer(ctx context.Context, req *connect.Request[afterdarkv1.LookupStashDBPerformerRequest]) (*connect.Response[afterdarkv1.LookupStashDBPerformerResponse], error) {
	p, err := h.svc.LookupPerformer(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.LookupStashDBPerformerResponse{Performer: stashDBPerformerToProto(*p)}), nil
}

// SearchPerformers implements afterdarkv1connect.StashDBServiceHandler.
func (h *StashDBHandler) SearchPerformers(ctx context.Context, req *connect.Request[afterdarkv1.SearchStashDBPerformersRequest]) (*connect.Response[afterdarkv1.SearchStashDBPerformersResponse], error) {
	performers, err := h.svc.SearchPerformers(ctx, req.Msg.GetTerm())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*afterdarkv1.StashDBPerformer, len(performers))
	for i, p := range performers {
		out[i] = stashDBPerformerToProto(p)
	}
	return connect.NewResponse(&afterdarkv1.SearchStashDBPerformersResponse{Performers: out}), nil
}

// stashDBPerformerToProto maps ports.Performer (internal/adapters/stashdb's
// own DTO) to its wire shape.
func stashDBPerformerToProto(p ports.Performer) *afterdarkv1.StashDBPerformer {
	urls := make([]*afterdarkv1.StashDBURL, len(p.URLs))
	for i, u := range p.URLs {
		urls[i] = &afterdarkv1.StashDBURL{
			Url: u.URL,
			Site: &afterdarkv1.StashDBSite{
				Id:   u.Site.ID,
				Name: u.Site.Name,
				Url:  u.Site.URL,
			},
		}
	}
	images := make([]*afterdarkv1.StashDBImage, len(p.Images))
	for i, img := range p.Images {
		images[i] = &afterdarkv1.StashDBImage{
			Id:     img.ID,
			Url:    img.URL,
			Width:  int32(img.Width),  //nolint:gosec // an image's pixel width is never remotely close to overflowing int32
			Height: int32(img.Height), //nolint:gosec // an image's pixel height is never remotely close to overflowing int32
		}
	}
	return &afterdarkv1.StashDBPerformer{
		Id:              p.ID,
		Name:            p.Name,
		Disambiguation:  p.Disambiguation,
		Aliases:         p.Aliases,
		Gender:          p.Gender,
		Urls:            urls,
		BirthDate:       p.BirthDate,
		Ethnicity:       p.Ethnicity,
		Country:         p.Country,
		EyeColor:        p.EyeColor,
		HairColor:       p.HairColor,
		Height:          int32(p.Height), //nolint:gosec // a performer's height in cm is never remotely close to overflowing int32
		CupSize:         p.CupSize,
		BandSize:        int32(p.BandSize),  //nolint:gosec // never remotely close to overflowing int32
		WaistSize:       int32(p.WaistSize), //nolint:gosec // never remotely close to overflowing int32
		HipSize:         int32(p.HipSize),   //nolint:gosec // never remotely close to overflowing int32
		BreastType:      p.BreastType,
		CareerStartYear: int32(p.CareerStartYear), //nolint:gosec // a calendar year is never remotely close to overflowing int32
		CareerEndYear:   int32(p.CareerEndYear),   //nolint:gosec // a calendar year is never remotely close to overflowing int32
		Images:          images,
		IsFavorite:      p.IsFavorite,
		Deleted:         p.Deleted,
		MergedIds:       p.MergedIDs,
	}
}
