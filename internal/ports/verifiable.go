package ports

import "context"

// Verifiable is implemented by MetadataSource adapters that can test their own
// connectivity and credentials. Verify makes a minimal read-only call to the
// external service; a nil return means the source is reachable and the
// configured credentials are accepted.
type Verifiable interface {
	Verify(ctx context.Context) error
}
