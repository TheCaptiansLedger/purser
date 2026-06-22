package stashdb

import "context"

// Verify confirms the configured API key is accepted by StashDB.
func (a *Adapter) Verify(ctx context.Context) error {
	_, err := a.SearchStudios(ctx, "test", 1)
	return err
}
