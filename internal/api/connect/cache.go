package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/cache/v1/cachev1connect"
	"purser/internal/service"

	"connectrpc.com/connect"

	cachev1 "purser/gen/go/purser/cache/v1"
)

// cacheService is the narrow interface CacheHandler depends on — see
// personService for the DIP convention this follows.
type cacheService interface {
	ListCacheStats(ctx context.Context) ([]service.CacheStats, error)
	FlushCache(ctx context.Context, name string) ([]string, error)
}

// CacheHandler implements cachev1connect.CacheServiceHandler — a thin
// request/response translator per docs/adr/0011-api-design.md, zero
// business logic.
type CacheHandler struct {
	cachev1connect.UnimplementedCacheServiceHandler
	svc    cacheService
	logger *slog.Logger
}

// NewCacheHandler constructs a CacheHandler backed by svc.
func NewCacheHandler(svc cacheService, logger *slog.Logger) *CacheHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CacheHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "CacheService")}
}

// ListCacheStats implements cachev1connect.CacheServiceHandler.
func (h *CacheHandler) ListCacheStats(ctx context.Context, _ *connect.Request[cachev1.ListCacheStatsRequest]) (*connect.Response[cachev1.ListCacheStatsResponse], error) {
	stats, err := h.svc.ListCacheStats(ctx)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&cachev1.ListCacheStatsResponse{Caches: cacheStatsSliceToProto(stats)}), nil
}

// FlushCache implements cachev1connect.CacheServiceHandler. An empty name
// flushes every registered cache — see the FlushCacheRequest proto doc
// comment.
func (h *CacheHandler) FlushCache(ctx context.Context, req *connect.Request[cachev1.FlushCacheRequest]) (*connect.Response[cachev1.FlushCacheResponse], error) {
	flushed, err := h.svc.FlushCache(ctx, req.Msg.GetName())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	h.logger.InfoContext(ctx, "cache flushed", "caches", flushed)
	return connect.NewResponse(&cachev1.FlushCacheResponse{Flushed: flushed}), nil
}
