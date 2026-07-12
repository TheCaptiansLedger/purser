package domain

import "testing"

func validExternalID() ExternalID {
	return ExternalID{
		EntityType: EntityTypePerson,
		EntityID:   "p1",
		Source:     ExternalIDSourceStashDB,
		Value:      "90a42491-f3f6-4764-8da8-564be11140f6",
	}
}

func TestExternalID_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ExternalID)
		wantErr bool
	}{
		{"valid", func(_ *ExternalID) {}, false},
		{"missing EntityType", func(e *ExternalID) { e.EntityType = "" }, true},
		{"invalid EntityType", func(e *ExternalID) { e.EntityType = "planet" }, true},
		{"missing EntityID", func(e *ExternalID) { e.EntityID = "" }, true},
		{"missing Source", func(e *ExternalID) { e.Source = "" }, true},
		{"missing Value", func(e *ExternalID) { e.Value = "" }, true},
		{"any non-empty Source is valid (open enum)", func(e *ExternalID) {
			e.Source = ExternalIDSource("a-provider-nobody-has-added-yet")
		}, false},
		{"group entity type is valid", func(e *ExternalID) { e.EntityType = EntityTypeGroup }, false},
		{"item entity type is valid", func(e *ExternalID) { e.EntityType = EntityTypeItem }, false},
		{"library_entry entity type is valid", func(e *ExternalID) { e.EntityType = EntityTypeLibraryEntry }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validExternalID()
			tt.mutate(&e)
			err := e.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
