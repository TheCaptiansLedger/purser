package fanart

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
)

// Verify confirms the configured API key is accepted by fanart.tv.
// Uses The Beatles' well-known MBID as the probe target.
func (a *Adapter) Verify(ctx context.Context) error {
	_, err := a.FindByExternalID(ctx, domain.ContentTypeMusic, "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	return err
}
