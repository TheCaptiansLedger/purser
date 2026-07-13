package domain

import "testing"

func validTagAssignment() TagAssignment {
	return TagAssignment{
		TagID:      "t1",
		EntityType: EntityTypePerson,
		EntityID:   "p1",
	}
}

func TestTagAssignment_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*TagAssignment)
		wantErr bool
	}{
		{"valid", func(_ *TagAssignment) {}, false},
		{"missing TagID", func(ta *TagAssignment) { ta.TagID = "" }, true},
		{"missing EntityType", func(ta *TagAssignment) { ta.EntityType = "" }, true},
		{"invalid EntityType", func(ta *TagAssignment) { ta.EntityType = "planet" }, true},
		{"missing EntityID", func(ta *TagAssignment) { ta.EntityID = "" }, true},
		{"group entity type is valid", func(ta *TagAssignment) { ta.EntityType = EntityTypeGroup }, false},
		{"item entity type is valid", func(ta *TagAssignment) { ta.EntityType = EntityTypeItem }, false},
		{"library_entry entity type is valid", func(ta *TagAssignment) { ta.EntityType = EntityTypeLibraryEntry }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := validTagAssignment()
			tt.mutate(&ta)
			err := ta.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
