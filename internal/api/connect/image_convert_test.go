package apiconnect

import (
	"purser/internal/domain"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestImageRoundTrip(t *testing.T) {
	if imageToProto(nil) != nil {
		t.Fatal("imageToProto(nil) did not return nil")
	}
	if imageFromProto(nil) == nil {
		t.Fatal("imageFromProto(nil) returned nil, want a zero-value Image")
	}

	img := &domain.Image{
		ID: "i1", OwnerType: "person", OwnerID: "p1", ImageType: domain.ImageTypePoster,
		URL: "https://example.com/i.jpg", Width: 100, Height: 200, Source: "stashdb", Priority: 1,
	}
	got := imageFromProto(imageToProto(img))
	if got.ID != img.ID || got.OwnerType != img.OwnerType || got.OwnerID != img.OwnerID ||
		got.ImageType != img.ImageType || got.URL != img.URL || got.Width != img.Width ||
		got.Height != img.Height || got.Source != img.Source || got.Priority != img.Priority {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
}

func TestApplyImageFieldMask(t *testing.T) {
	existing := &domain.Image{ID: "i1", URL: "https://example.com/original.jpg", Priority: 1}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyImageFieldMask(existing, &v1.Image{Id: "i1", Url: "https://example.com/new.jpg"}, nil)
		if got.URL != "https://example.com/new.jpg" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.Image{
			OwnerType: "afterdark.performer_profile", OwnerId: "pp1", ImageType: "hero",
			Url: "https://example.com/n.jpg", Width: 50, Height: 60, Source: "tpdb", Priority: 9,
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"owner_type", "owner_id", "image_type", "url", "width", "height", "source", "priority", "unrecognized",
		}}
		got := applyImageFieldMask(existing, full, mask)
		if got.OwnerType != "afterdark.performer_profile" || got.OwnerID != "pp1" || got.ImageType != domain.ImageType("hero") ||
			got.URL != "https://example.com/n.jpg" || got.Width != 50 || got.Height != 60 || got.Source != "tpdb" || got.Priority != 9 {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
