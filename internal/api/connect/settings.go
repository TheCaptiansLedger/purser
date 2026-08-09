package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/settings/v1/settingsv1connect"
	"purser/internal/service"

	"connectrpc.com/connect"

	settingsv1 "purser/gen/go/purser/settings/v1"
)

// settingsService is the narrow interface SettingsHandler depends on —
// see personService for the DIP convention this follows.
type settingsService interface {
	GetSettings(ctx context.Context) ([]service.SettingView, error)
	UpdateSettings(ctx context.Context, values map[string]string, keys []string) ([]service.SettingView, error)
	ResetSetting(ctx context.Context, key string) (service.SettingView, error)
}

// SettingsHandler implements settingsv1connect.SettingsServiceHandler — a
// thin request/response translator per docs/adr/0011-api-design.md, zero
// business logic. See docs/adr/0028-layered-settings.md for what it
// exposes.
type SettingsHandler struct {
	settingsv1connect.UnimplementedSettingsServiceHandler
	svc    settingsService
	logger *slog.Logger
}

// NewSettingsHandler constructs a SettingsHandler backed by svc.
func NewSettingsHandler(svc settingsService, logger *slog.Logger) *SettingsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SettingsHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "SettingsService")}
}

// GetSettings implements settingsv1connect.SettingsServiceHandler.
func (h *SettingsHandler) GetSettings(ctx context.Context, _ *connect.Request[settingsv1.GetSettingsRequest]) (*connect.Response[settingsv1.GetSettingsResponse], error) {
	views, err := h.svc.GetSettings(ctx)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&settingsv1.GetSettingsResponse{Settings: settingsToProto(views)}), nil
}

// UpdateSettings implements settingsv1connect.SettingsServiceHandler.
// update_mask names which dotted config keys to write; values holds each
// named key's new JSON-encoded value — see the UpdateSettingsRequest proto
// doc comment for why update_mask is a plain repeated string rather than
// google.protobuf.FieldMask.
func (h *SettingsHandler) UpdateSettings(ctx context.Context, req *connect.Request[settingsv1.UpdateSettingsRequest]) (*connect.Response[settingsv1.UpdateSettingsResponse], error) {
	views, err := h.svc.UpdateSettings(ctx, req.Msg.GetValues(), req.Msg.GetUpdateMask())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&settingsv1.UpdateSettingsResponse{Settings: settingsToProto(views)}), nil
}

// ResetSetting implements settingsv1connect.SettingsServiceHandler.
func (h *SettingsHandler) ResetSetting(ctx context.Context, req *connect.Request[settingsv1.ResetSettingRequest]) (*connect.Response[settingsv1.ResetSettingResponse], error) {
	view, err := h.svc.ResetSetting(ctx, req.Msg.GetKey())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&settingsv1.ResetSettingResponse{Setting: settingToProto(view)}), nil
}
