package ports

import (
	"context"
	"encoding/json"
)

// GitHubProxy proxies GitHub API calls server-side with caching.
// All contributor endpoints return a flat JSON array of commit objects,
// regardless of whether the underlying call was a compare or commits request.
type GitHubProxy interface {
	// Issues returns GitHub issues JSON for the given state. If label is non-empty
	// only issues carrying that label are returned.
	Issues(ctx context.Context, state, label string) (json.RawMessage, error)
	// Releases returns GitHub releases JSON.
	Releases(ctx context.Context) (json.RawMessage, error)
	// Contributors returns a JSON array of commit objects spanning base..head.
	// If base is empty, all commits reachable from head are returned.
	Contributors(ctx context.Context, base, head string) (json.RawMessage, error)
}
