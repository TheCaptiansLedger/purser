package apiconnect

import (
	"purser/internal/domain"
	"testing"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestEntityTypeRoundTrip(t *testing.T) {
	cases := []struct {
		domainVal domain.EntityType
		protoVal  v1.EntityType
	}{
		{domain.EntityTypeLibraryEntry, v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY},
		{domain.EntityTypeGroup, v1.EntityType_ENTITY_TYPE_GROUP},
		{domain.EntityTypeItem, v1.EntityType_ENTITY_TYPE_ITEM},
		{domain.EntityTypePerson, v1.EntityType_ENTITY_TYPE_PERSON},
	}
	for _, c := range cases {
		if got := entityTypeToProto(c.domainVal); got != c.protoVal {
			t.Errorf("entityTypeToProto(%v) = %v, want %v", c.domainVal, got, c.protoVal)
		}
		if got := entityTypeFromProto(c.protoVal); got != c.domainVal {
			t.Errorf("entityTypeFromProto(%v) = %v, want %v", c.protoVal, got, c.domainVal)
		}
	}
	if got := entityTypeFromProto(v1.EntityType_ENTITY_TYPE_UNSPECIFIED); got != domain.EntityType("") {
		t.Errorf("entityTypeFromProto(UNSPECIFIED) = %q, want zero value", got)
	}
	if got := entityTypeToProto(domain.EntityType("bogus")); got != v1.EntityType_ENTITY_TYPE_UNSPECIFIED {
		t.Errorf("entityTypeToProto(bogus) = %v, want ENTITY_TYPE_UNSPECIFIED", got)
	}
}

func TestExternalIDRoundTrip(t *testing.T) {
	if externalIDToProto(nil) != nil {
		t.Fatal("externalIDToProto(nil) did not return nil")
	}
	if externalIDFromProto(nil) == nil {
		t.Fatal("externalIDFromProto(nil) returned nil, want a zero-value ExternalID")
	}

	e := &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: "stashdb", Value: "v1"}
	got := externalIDFromProto(externalIDToProto(e))
	if got.EntityType != e.EntityType || got.EntityID != e.EntityID || got.Source != e.Source || got.Value != e.Value {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
}
