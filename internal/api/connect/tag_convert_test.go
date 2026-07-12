package apiconnect

import (
	"purser/internal/domain"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestTagScopeRoundTrip(t *testing.T) {
	cases := []struct {
		domainVal domain.TagScope
		protoVal  v1.TagScope
	}{
		{domain.TagScopeUser, v1.TagScope_TAG_SCOPE_USER},
		{domain.TagScopeMetadata, v1.TagScope_TAG_SCOPE_METADATA},
	}
	for _, c := range cases {
		if got := tagScopeToProto(c.domainVal); got != c.protoVal {
			t.Errorf("tagScopeToProto(%v) = %v, want %v", c.domainVal, got, c.protoVal)
		}
		if got := tagScopeFromProto(c.protoVal); got != c.domainVal {
			t.Errorf("tagScopeFromProto(%v) = %v, want %v", c.protoVal, got, c.domainVal)
		}
	}
	if got := tagScopeFromProto(v1.TagScope_TAG_SCOPE_UNSPECIFIED); got != domain.TagScope("") {
		t.Errorf("tagScopeFromProto(UNSPECIFIED) = %q, want zero value", got)
	}
	if got := tagScopeToProto(domain.TagScope("bogus")); got != v1.TagScope_TAG_SCOPE_UNSPECIFIED {
		t.Errorf("tagScopeToProto(bogus) = %v, want TAG_SCOPE_UNSPECIFIED", got)
	}
}

func TestTagRoundTrip(t *testing.T) {
	if tagToProto(nil) != nil {
		t.Fatal("tagToProto(nil) did not return nil")
	}
	if tagFromProto(nil) == nil {
		t.Fatal("tagFromProto(nil) returned nil, want a zero-value Tag")
	}

	tag := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata, Category: "Action"}
	got := tagFromProto(tagToProto(tag))
	if got.ID != tag.ID || got.Key != tag.Key || got.Value != tag.Value || got.Scope != tag.Scope || got.Category != tag.Category {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
}

func TestApplyTagFieldMask(t *testing.T) {
	existing := &domain.Tag{ID: "t1", Key: "genre", Value: "Original", Scope: domain.TagScopeMetadata, Category: "Original Category"}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyTagFieldMask(existing, &v1.Tag{Id: "t1", Value: "New"}, nil)
		if got.Value != "New" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.Tag{Key: "k2", Value: "v2", Scope: v1.TagScope_TAG_SCOPE_USER, Category: "c2"}
		mask := &fieldmaskpb.FieldMask{Paths: []string{"key", "value", "scope", "category", "unrecognized"}}
		got := applyTagFieldMask(existing, full, mask)
		if got.Key != "k2" || got.Value != "v2" || got.Scope != domain.TagScopeUser || got.Category != "c2" {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
