package domain

import "testing"

func validCollection() Collection {
	return Collection{
		ID:   "c1",
		Name: "The Matrix Collection",
		Kind: "franchise",
	}
}

func TestCollection_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Collection)
		wantErr bool
	}{
		{"valid", func(_ *Collection) {}, false},
		{"missing ID", func(c *Collection) { c.ID = "" }, true},
		{"missing Name", func(c *Collection) { c.Name = "" }, true},
		{"missing Kind", func(c *Collection) { c.Kind = "" }, true},
		{"any non-empty Kind is valid (open string)", func(c *Collection) {
			c.Kind = "book_series"
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCollection()
			tt.mutate(&c)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func validCollectionMembership() CollectionMembership {
	return CollectionMembership{
		CollectionID: "c1",
		EntityType:   EntityTypeLibraryEntry,
		EntityID:     "le1",
		Position:     "1",
	}
}

func TestCollectionMembership_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CollectionMembership)
		wantErr bool
	}{
		{"valid", func(_ *CollectionMembership) {}, false},
		{"missing CollectionID", func(m *CollectionMembership) { m.CollectionID = "" }, true},
		{"missing EntityType", func(m *CollectionMembership) { m.EntityType = "" }, true},
		{"invalid EntityType", func(m *CollectionMembership) { m.EntityType = "item" }, true},
		{"missing EntityID", func(m *CollectionMembership) { m.EntityID = "" }, true},
		{"group entity type is valid", func(m *CollectionMembership) { m.EntityType = EntityTypeGroup }, false},
		{"fractional position is valid", func(m *CollectionMembership) { m.Position = "1.5" }, false},
		{"empty position is valid (optional)", func(m *CollectionMembership) { m.Position = "" }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validCollectionMembership()
			tt.mutate(&m)
			err := m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
