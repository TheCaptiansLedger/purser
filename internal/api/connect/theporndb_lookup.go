package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// thePornDBLookupService is the narrow interface ThePornDBHandler depends
// on — see personService for the DIP convention this follows.
type thePornDBLookupService interface {
	LookupPerformer(ctx context.Context, id string) (*ports.TPDBPerformer, error)
	SearchPerformers(ctx context.Context, term string) ([]ports.TPDBPerformer, error)
}

// ThePornDBHandler implements afterdarkv1connect.ThePornDBServiceHandler —
// see proto/purser/afterdark/v1/theporndb.proto's own doc comment for why
// this exists.
type ThePornDBHandler struct {
	afterdarkv1connect.UnimplementedThePornDBServiceHandler
	svc    thePornDBLookupService
	logger *slog.Logger
}

// NewThePornDBHandler constructs a ThePornDBHandler backed by svc.
func NewThePornDBHandler(svc thePornDBLookupService, logger *slog.Logger) *ThePornDBHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ThePornDBHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "ThePornDBService")}
}

// LookupPerformer implements afterdarkv1connect.ThePornDBServiceHandler.
func (h *ThePornDBHandler) LookupPerformer(ctx context.Context, req *connect.Request[afterdarkv1.LookupThePornDBPerformerRequest]) (*connect.Response[afterdarkv1.LookupThePornDBPerformerResponse], error) {
	p, err := h.svc.LookupPerformer(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.LookupThePornDBPerformerResponse{Performer: tpdbPerformerToProto(*p)}), nil
}

// SearchPerformers implements afterdarkv1connect.ThePornDBServiceHandler.
func (h *ThePornDBHandler) SearchPerformers(ctx context.Context, req *connect.Request[afterdarkv1.SearchThePornDBPerformersRequest]) (*connect.Response[afterdarkv1.SearchThePornDBPerformersResponse], error) {
	performers, err := h.svc.SearchPerformers(ctx, req.Msg.GetTerm())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*afterdarkv1.TPDBPerformer, len(performers))
	for i, p := range performers {
		out[i] = tpdbPerformerToProto(p)
	}
	return connect.NewResponse(&afterdarkv1.SearchThePornDBPerformersResponse{Performers: out}), nil
}

// tpdbPerformerToProto maps ports.TPDBPerformer (internal/adapters/theporndb's
// own DTO) to its wire shape. parent is mapped one level deep only, mirroring
// the Go DTO's own shallow-Parent convention (see theporndb.proto's doc
// comment) — a persona's own Parent is never itself expected to carry a
// further-nested Parent, so recursing once is enough.
func tpdbPerformerToProto(p ports.TPDBPerformer) *afterdarkv1.TPDBPerformer {
	posters := make([]*afterdarkv1.TPDBImage, len(p.Posters))
	for i, img := range p.Posters {
		posters[i] = &afterdarkv1.TPDBImage{
			Id:    int32(img.ID), //nolint:gosec // ThePornDB's own image IDs are never remotely close to overflowing int32
			Url:   img.URL,
			Size:  int32(img.Size),  //nolint:gosec // never remotely close to overflowing int32
			Order: int32(img.Order), //nolint:gosec // never remotely close to overflowing int32
		}
	}

	out := &afterdarkv1.TPDBPerformer{
		Id:             p.ID,
		LegacyId:       int32(p.LegacyID), //nolint:gosec // ThePornDB's own legacy numeric IDs are never remotely close to overflowing int32
		Slug:           p.Slug,
		Name:           p.Name,
		FullName:       p.FullName,
		Disambiguation: p.Disambiguation,
		Bio:            p.Bio,
		Rating:         p.Rating,
		IsParent:       p.IsParent,
		Image:          p.Image,
		Thumbnail:      p.Thumbnail,
		Face:           p.Face,
		Posters:        posters,
		Aliases:        p.Aliases,
		Extras:         tpdbPerformerExtrasToProto(p.Extras),
	}
	if p.Parent != nil {
		out.Parent = tpdbPerformerToProto(*p.Parent)
	}
	return out
}

// tpdbPerformerExtrasToProto maps ports.TPDBPerformerExtras to its wire
// shape. Links (ports.TPDBLinks, a map[string]string with a custom
// unmarshaler already normalizing ThePornDB's confirmed-live "[]"
// empty-array quirk) converts directly — a nil map marshals to an empty
// map on the wire, never that quirk leaking through.
func tpdbPerformerExtrasToProto(e ports.TPDBPerformerExtras) *afterdarkv1.TPDBPerformerExtras {
	return &afterdarkv1.TPDBPerformerExtras{
		Gender:            e.Gender,
		Birthday:          e.Birthday,
		BirthdayTimestamp: e.BirthdayTimestamp,
		Birthplace:        e.Birthplace,
		BirthplaceCode:    e.BirthplaceCode,
		Astrology:         e.Astrology,
		Ethnicity:         e.Ethnicity,
		Nationality:       e.Nationality,
		HairColour:        e.HairColour,
		EyeColour:         e.EyeColour,
		Weight:            e.Weight,
		Height:            e.Height,
		Measurements:      e.Measurements,
		CupSize:           e.CupSize,
		Tattoos:           e.Tattoos,
		Piercings:         e.Piercings,
		Waist:             e.Waist,
		Hips:              e.Hips,
		FakeBoobs:         e.FakeBoobs,
		SameSexOnly:       e.SameSexOnly,
		CareerStartYear:   int32(e.CareerStartYear), //nolint:gosec // a calendar year is never remotely close to overflowing int32
		CareerEndYear:     int32(e.CareerEndYear),   //nolint:gosec // a calendar year is never remotely close to overflowing int32
		Links:             map[string]string(e.Links),
	}
}
