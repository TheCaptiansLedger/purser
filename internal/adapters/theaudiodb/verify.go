package theaudiodb

import "context"

// Verify confirms the source is reachable with the configured API key.
func (a *Adapter) Verify(ctx context.Context) error {
	_, err := a.SearchStudios(ctx, "Radiohead", 1)
	return err
}
