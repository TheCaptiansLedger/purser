package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakeTemplateDataBuilder is a minimal ports.TemplateDataBuilder double for
// exercising TemplateDataBuilderRegistry's dispatch — records the item it
// was called with and returns a fixed map/error so tests can tell it apart
// from service.NoopTemplateDataBuilder's empty-map behavior.
type fakeTemplateDataBuilder struct {
	contentTypes []domain.ContentType
	data         map[string]any
	err          error

	called bool
	item   *domain.Item
}

func (f *fakeTemplateDataBuilder) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeTemplateDataBuilder) BuildTemplateData(_ context.Context, item *domain.Item) (map[string]any, error) {
	f.called = true
	f.item = item
	return f.data, f.err
}

func TestTemplateDataBuilderRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakeTemplateDataBuilder{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		data:         map[string]any{"ArtistName": "Test Artist"},
	}
	registry := service.NewTemplateDataBuilderRegistry(music)

	item := &domain.Item{ID: "item1"}
	data, err := registry.BuildTemplateData(context.Background(), domain.ContentTypeMusic, item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if !music.called {
		t.Fatal("BuildTemplateData did not dispatch to the registered fakeTemplateDataBuilder")
	}
	if music.item != item {
		t.Fatalf("BuildTemplateData called with item %+v, want %+v", music.item, item)
	}
	if data["ArtistName"] != "Test Artist" {
		t.Fatalf("BuildTemplateData returned %+v, want ArtistName=Test Artist", data)
	}
}

func TestTemplateDataBuilderRegistry_PropagatesBuilderError(t *testing.T) {
	wantErr := errors.New("build failed")
	music := &fakeTemplateDataBuilder{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, err: wantErr}
	registry := service.NewTemplateDataBuilderRegistry(music)

	_, err := registry.BuildTemplateData(context.Background(), domain.ContentTypeMusic, &domain.Item{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("BuildTemplateData returned error %v, want %v", err, wantErr)
	}
}

func TestTemplateDataBuilderRegistry_FallsBackToNoopWhenUnregistered(t *testing.T) {
	music := &fakeTemplateDataBuilder{contentTypes: []domain.ContentType{domain.ContentTypeMusic}}
	registry := service.NewTemplateDataBuilderRegistry(music)

	data, err := registry.BuildTemplateData(context.Background(), domain.ContentTypeMovie, &domain.Item{})
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if music.called {
		t.Fatal("BuildTemplateData dispatched to the music fakeTemplateDataBuilder for an unregistered content type")
	}
	if len(data) != 0 {
		t.Fatalf("BuildTemplateData fallback returned %+v, want empty map", data)
	}
}

func TestTemplateDataBuilderRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewTemplateDataBuilderRegistry()

	if _, err := registry.BuildTemplateData(context.Background(), domain.ContentTypeMusic, &domain.Item{}); err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
}

func TestNoopTemplateDataBuilder_ReturnsEmptyMap(t *testing.T) {
	b := service.NoopTemplateDataBuilder{}
	data, err := b.BuildTemplateData(context.Background(), &domain.Item{})
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("BuildTemplateData = %+v, want empty map", data)
	}
}

func TestNoopTemplateDataBuilder_ContentTypesReturnsNil(t *testing.T) {
	b := service.NoopTemplateDataBuilder{}
	if got := b.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
