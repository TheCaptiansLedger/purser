package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeTagService struct {
	byID      map[string]*domain.Tag
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeTagService() *fakeTagService {
	return &fakeTagService{byID: make(map[string]*domain.Tag)}
}

func (f *fakeTagService) Create(_ context.Context, t *domain.Tag) (*domain.Tag, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeTagService) Get(_ context.Context, id string) (*domain.Tag, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	t, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return t, nil
}

func (f *fakeTagService) Update(_ context.Context, t *domain.Tag) (*domain.Tag, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeTagService) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeTagService) List(_ context.Context, _ int, _ string) ([]*domain.Tag, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	tags := make([]*domain.Tag, 0, len(f.byID))
	for _, t := range f.byID {
		tags = append(tags, t)
	}
	return tags, "", nil
}

func validProtoTag(id string) *v1.Tag {
	return &v1.Tag{Id: id, Key: "genre", Value: "action", Scope: v1.TagScope_TAG_SCOPE_METADATA}
}

func TestTagHandler_CreateTag(t *testing.T) {
	t.Run("valid request returns the created tag", func(t *testing.T) {
		svc := newFakeTagService()
		h := apiconnect.NewTagHandler(svc, nil)

		res, err := h.CreateTag(context.Background(), connect.NewRequest(&v1.CreateTagRequest{Tag: validProtoTag("t1")}))
		if err != nil {
			t.Fatalf("CreateTag returned error: %v", err)
		}
		if res.Msg.GetTag().GetId() != "t1" {
			t.Fatalf("CreateTag returned ID %q, want %q", res.Msg.GetTag().GetId(), "t1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeTagService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Value", Rule: "required", Value: ""}}}
		h := apiconnect.NewTagHandler(svc, nil)

		_, err := h.CreateTag(context.Background(), connect.NewRequest(&v1.CreateTagRequest{Tag: validProtoTag("t1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateTag with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestTagHandler_GetTag(t *testing.T) {
	svc := newFakeTagService()
	h := apiconnect.NewTagHandler(svc, nil)
	svc.byID["t1"] = &domain.Tag{ID: "t1", Key: "genre", Value: "Existing", Scope: domain.TagScopeMetadata}

	res, err := h.GetTag(context.Background(), connect.NewRequest(&v1.GetTagRequest{Id: "t1"}))
	if err != nil {
		t.Fatalf("GetTag returned error: %v", err)
	}
	if res.Msg.GetTag().GetValue() != "Existing" {
		t.Fatalf("GetTag returned Value %q, want %q", res.Msg.GetTag().GetValue(), "Existing")
	}

	_, err = h.GetTag(context.Background(), connect.NewRequest(&v1.GetTagRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetTag on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestTagHandler_UpdateTag(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeTagService()
		h := apiconnect.NewTagHandler(svc, nil)
		svc.byID["t1"] = &domain.Tag{ID: "t1", Key: "genre", Value: "Original", Scope: domain.TagScopeMetadata, Category: "Original Category"}

		req := &v1.UpdateTagRequest{
			Tag:        &v1.Tag{Id: "t1", Value: "New Value", Category: "Should be ignored"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"value"}},
		}
		res, err := h.UpdateTag(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateTag returned error: %v", err)
		}
		if res.Msg.GetTag().GetValue() != "New Value" {
			t.Fatalf("UpdateTag applied Value %q, want %q", res.Msg.GetTag().GetValue(), "New Value")
		}
		if res.Msg.GetTag().GetCategory() != "Original Category" {
			t.Fatalf("UpdateTag touched Category: got %q", res.Msg.GetTag().GetCategory())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeTagService()
		h := apiconnect.NewTagHandler(svc, nil)

		_, err := h.UpdateTag(context.Background(), connect.NewRequest(&v1.UpdateTagRequest{Tag: &v1.Tag{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateTag on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestTagHandler_DeleteTag(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeTagService()
		h := apiconnect.NewTagHandler(svc, nil)
		svc.byID["t1"] = &domain.Tag{ID: "t1"}

		if _, err := h.DeleteTag(context.Background(), connect.NewRequest(&v1.DeleteTagRequest{Id: "t1"})); err != nil {
			t.Fatalf("DeleteTag returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeTagService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewTagHandler(svc, nil)

		_, err := h.DeleteTag(context.Background(), connect.NewRequest(&v1.DeleteTagRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteTag on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestTagHandler_ListTags(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeTagService()
		h := apiconnect.NewTagHandler(svc, nil)
		svc.byID["t1"] = &domain.Tag{ID: "t1"}
		svc.byID["t2"] = &domain.Tag{ID: "t2"}

		res, err := h.ListTags(context.Background(), connect.NewRequest(&v1.ListTagsRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListTags returned error: %v", err)
		}
		if len(res.Msg.GetTags()) != 2 {
			t.Fatalf("ListTags returned %d tags, want 2", len(res.Msg.GetTags()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeTagService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewTagHandler(svc, nil)

		_, err := h.ListTags(context.Background(), connect.NewRequest(&v1.ListTagsRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListTags with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
