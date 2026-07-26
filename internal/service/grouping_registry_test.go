package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeGrouping is a minimal ports.Grouping double for exercising
// GroupingRegistry's dispatch — every registered path maps to a fixed
// GroupKey/DiscNumber regardless of input, so tests can tell it apart from
// service.IdentityGrouping's path-is-its-own-key behavior.
type fakeGrouping struct {
	contentTypes []domain.ContentType
	err          error
}

func (f *fakeGrouping) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeGrouping) GroupKeys(_ context.Context, paths []string) (map[string]ports.GroupingResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := make(map[string]ports.GroupingResult, len(paths))
	for _, p := range paths {
		result[p] = ports.GroupingResult{GroupKey: "fixed-group", DiscNumber: 1}
	}
	return result, nil
}

func TestGroupingRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakeGrouping{contentTypes: []domain.ContentType{domain.ContentTypeMusic}}
	registry := service.NewGroupingRegistry(music)

	got, err := registry.GroupKeys(context.Background(), domain.ContentTypeMusic, []string{"/media/a.flac"})
	if err != nil {
		t.Fatalf("GroupKeys returned error: %v", err)
	}
	want := ports.GroupingResult{GroupKey: "fixed-group", DiscNumber: 1}
	if got["/media/a.flac"] != want {
		t.Fatalf("GroupKeys()[/media/a.flac] = %+v, want %+v", got["/media/a.flac"], want)
	}
}

func TestGroupingRegistry_FallsBackToIdentityGroupingWhenUnregistered(t *testing.T) {
	music := &fakeGrouping{contentTypes: []domain.ContentType{domain.ContentTypeMusic}}
	registry := service.NewGroupingRegistry(music)

	got, err := registry.GroupKeys(context.Background(), domain.ContentTypeMovie, []string{"/media/a.mkv", "/media/b.mkv"})
	if err != nil {
		t.Fatalf("GroupKeys returned error: %v", err)
	}
	for _, path := range []string{"/media/a.mkv", "/media/b.mkv"} {
		want := ports.GroupingResult{GroupKey: path, DiscNumber: 0}
		if got[path] != want {
			t.Errorf("GroupKeys()[%q] = %+v, want %+v (IdentityGrouping fallback)", path, got[path], want)
		}
	}
}

func TestGroupingRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewGroupingRegistry()

	got, err := registry.GroupKeys(context.Background(), domain.ContentTypeMusic, []string{"/media/a.flac"})
	if err != nil {
		t.Fatalf("GroupKeys returned error: %v", err)
	}
	want := ports.GroupingResult{GroupKey: "/media/a.flac", DiscNumber: 0}
	if got["/media/a.flac"] != want {
		t.Fatalf("GroupKeys()[/media/a.flac] = %+v, want %+v", got["/media/a.flac"], want)
	}
}

func TestGroupingRegistry_PropagatesImplementationError(t *testing.T) {
	wantErr := errors.New("boom")
	music := &fakeGrouping{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, err: wantErr}
	registry := service.NewGroupingRegistry(music)

	_, err := registry.GroupKeys(context.Background(), domain.ContentTypeMusic, []string{"/media/a.flac"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("GroupKeys returned %v, want wrapping %v", err, wantErr)
	}
}

func TestIdentityGrouping_MapsEveryPathToItself(t *testing.T) {
	g := service.IdentityGrouping{}

	got, err := g.GroupKeys(context.Background(), []string{"/media/a.flac", "/media/b.flac"})
	if err != nil {
		t.Fatalf("GroupKeys returned error: %v", err)
	}
	for _, path := range []string{"/media/a.flac", "/media/b.flac"} {
		want := ports.GroupingResult{GroupKey: path, DiscNumber: 0}
		if got[path] != want {
			t.Errorf("GroupKeys()[%q] = %+v, want %+v", path, got[path], want)
		}
	}
}

func TestIdentityGrouping_ContentTypesReturnsNil(t *testing.T) {
	g := service.IdentityGrouping{}
	if got := g.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
