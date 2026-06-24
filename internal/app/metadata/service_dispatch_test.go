package metadata

import (
	"context"
	"errors"
	"purser/internal/domain"
	"testing"
)

func TestRefreshForKind_UnknownKind_ReturnsErrUnknownJob(t *testing.T) {
	svc := &Service{}
	err := svc.refreshForKind(context.Background(), domain.Kind("bogus"), "some-id", nil)
	if err == nil {
		t.Fatal("expected error for unrecognised kind, got nil")
	}
	if !errors.Is(err, ErrUnknownJob) {
		t.Errorf("err = %v, want ErrUnknownJob", err)
	}
}
