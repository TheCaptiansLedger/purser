package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"testing"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeBrowseService struct {
	scenes   []*domain.Item
	views    []*afterdark.PerformerView
	listErr  error
	gotFirst string
}

func newFakeBrowseService() *fakeBrowseService {
	return &fakeBrowseService{}
}

func (f *fakeBrowseService) ListScenesInNetwork(_ context.Context, networkID string, _ int, _ string) ([]*domain.Item, string, error) {
	f.gotFirst = networkID
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.scenes, "", nil
}

func (f *fakeBrowseService) ListScenesForPerformer(_ context.Context, personID string, _ int, _ string) ([]*domain.Item, string, error) {
	f.gotFirst = personID
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.scenes, "", nil
}

func (f *fakeBrowseService) ListPerformersForScene(_ context.Context, itemID string, _ int, _ string) ([]*afterdark.PerformerView, string, error) {
	f.gotFirst = itemID
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.views, "", nil
}

func (f *fakeBrowseService) ListPerformersForStudio(_ context.Context, libraryEntryID string, _ int, _ string) ([]*afterdark.PerformerView, string, error) {
	f.gotFirst = libraryEntryID
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.views, "", nil
}

func (f *fakeBrowseService) ListPerformersForNetwork(_ context.Context, networkID string, _ int, _ string) ([]*afterdark.PerformerView, string, error) {
	f.gotFirst = networkID
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.views, "", nil
}

func (f *fakeBrowseService) ListPerformers(_ context.Context, _ int, _ string) ([]*afterdark.PerformerView, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.views, "", nil
}

func TestBrowseHandler_ListScenesInNetwork(t *testing.T) {
	svc := newFakeBrowseService()
	svc.scenes = []*domain.Item{{ID: "scene1", ContentType: domain.ContentTypeAdult, Status: domain.ItemStatusWanted}}
	h := apiconnect.NewBrowseHandler(svc, nil)

	res, err := h.ListScenesInNetwork(context.Background(), connect.NewRequest(&afterdarkv1.ListScenesInNetworkRequest{NetworkId: "network1", PageSize: 10}))
	if err != nil {
		t.Fatalf("ListScenesInNetwork returned error: %v", err)
	}
	if svc.gotFirst != "network1" {
		t.Fatalf("ListScenesInNetwork passed networkId %q, want %q", svc.gotFirst, "network1")
	}
	if len(res.Msg.GetScenes()) != 1 || res.Msg.GetScenes()[0].GetId() != "scene1" {
		t.Fatalf("ListScenesInNetwork returned %v, want [scene1]", res.Msg.GetScenes())
	}
}

func TestBrowseHandler_ListScenesForPerformer(t *testing.T) {
	svc := newFakeBrowseService()
	svc.scenes = []*domain.Item{{ID: "scene1", ContentType: domain.ContentTypeAdult, Status: domain.ItemStatusWanted}}
	h := apiconnect.NewBrowseHandler(svc, nil)

	res, err := h.ListScenesForPerformer(context.Background(), connect.NewRequest(&afterdarkv1.ListScenesForPerformerRequest{PersonId: "p1", PageSize: 10}))
	if err != nil {
		t.Fatalf("ListScenesForPerformer returned error: %v", err)
	}
	if svc.gotFirst != "p1" {
		t.Fatalf("ListScenesForPerformer passed personId %q, want %q", svc.gotFirst, "p1")
	}
	if len(res.Msg.GetScenes()) != 1 {
		t.Fatalf("ListScenesForPerformer returned %d scenes, want 1", len(res.Msg.GetScenes()))
	}
}

func TestBrowseHandler_ListPerformersForScene(t *testing.T) {
	svc := newFakeBrowseService()
	svc.views = []*afterdark.PerformerView{{Person: &domain.Person{ID: "p1", Name: "Performer One"}, Profile: &afterdark.PerformerProfile{PersonID: "p1"}}}
	h := apiconnect.NewBrowseHandler(svc, nil)

	res, err := h.ListPerformersForScene(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformersForSceneRequest{ItemId: "scene1", PageSize: 10}))
	if err != nil {
		t.Fatalf("ListPerformersForScene returned error: %v", err)
	}
	if svc.gotFirst != "scene1" {
		t.Fatalf("ListPerformersForScene passed itemId %q, want %q", svc.gotFirst, "scene1")
	}
	if len(res.Msg.GetPerformers()) != 1 || res.Msg.GetPerformers()[0].GetPerson().GetId() != "p1" {
		t.Fatalf("ListPerformersForScene returned %v, want a single view for p1", res.Msg.GetPerformers())
	}
}

func TestBrowseHandler_ListPerformersForStudio(t *testing.T) {
	svc := newFakeBrowseService()
	svc.views = []*afterdark.PerformerView{{Person: &domain.Person{ID: "p1"}, Profile: &afterdark.PerformerProfile{PersonID: "p1"}}}
	h := apiconnect.NewBrowseHandler(svc, nil)

	res, err := h.ListPerformersForStudio(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformersForStudioRequest{LibraryEntryId: "studio1", PageSize: 10}))
	if err != nil {
		t.Fatalf("ListPerformersForStudio returned error: %v", err)
	}
	if svc.gotFirst != "studio1" {
		t.Fatalf("ListPerformersForStudio passed libraryEntryId %q, want %q", svc.gotFirst, "studio1")
	}
	if len(res.Msg.GetPerformers()) != 1 {
		t.Fatalf("ListPerformersForStudio returned %d performers, want 1", len(res.Msg.GetPerformers()))
	}
}

func TestBrowseHandler_ListPerformersForNetwork(t *testing.T) {
	svc := newFakeBrowseService()
	svc.views = []*afterdark.PerformerView{{Person: &domain.Person{ID: "p1"}, Profile: &afterdark.PerformerProfile{PersonID: "p1"}}}
	h := apiconnect.NewBrowseHandler(svc, nil)

	res, err := h.ListPerformersForNetwork(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformersForNetworkRequest{NetworkId: "network1", PageSize: 10}))
	if err != nil {
		t.Fatalf("ListPerformersForNetwork returned error: %v", err)
	}
	if svc.gotFirst != "network1" {
		t.Fatalf("ListPerformersForNetwork passed networkId %q, want %q", svc.gotFirst, "network1")
	}
	if len(res.Msg.GetPerformers()) != 1 {
		t.Fatalf("ListPerformersForNetwork returned %d performers, want 1", len(res.Msg.GetPerformers()))
	}
}

func TestBrowseHandler_ListPerformers(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeBrowseService()
		svc.views = []*afterdark.PerformerView{
			{Person: &domain.Person{ID: "p1"}, Profile: &afterdark.PerformerProfile{PersonID: "p1"}},
			{Person: &domain.Person{ID: "p2"}, Profile: &afterdark.PerformerProfile{PersonID: "p2"}},
		}
		h := apiconnect.NewBrowseHandler(svc, nil)

		res, err := h.ListPerformers(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformersRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListPerformers returned error: %v", err)
		}
		if len(res.Msg.GetPerformers()) != 2 {
			t.Fatalf("ListPerformers returned %d performers, want 2", len(res.Msg.GetPerformers()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeBrowseService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewBrowseHandler(svc, nil)

		_, err := h.ListPerformers(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformersRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListPerformers with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
