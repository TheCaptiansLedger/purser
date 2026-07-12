package domain

import "testing"

func validTag() Tag {
	return Tag{
		ID:    "t1",
		Key:   "genre",
		Value: "gonzo",
		Scope: TagScopeMetadata,
	}
}

func TestTag_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Tag)
		wantErr bool
	}{
		{"valid", func(_ *Tag) {}, false},
		{"valid with category", func(tg *Tag) { tg.Category = "ACTION" }, false},
		{"missing ID", func(tg *Tag) { tg.ID = "" }, true},
		{"missing Key", func(tg *Tag) { tg.Key = "" }, true},
		{"missing Value", func(tg *Tag) { tg.Value = "" }, true},
		{"missing Scope", func(tg *Tag) { tg.Scope = "" }, true},
		{"invalid Scope", func(tg *Tag) { tg.Scope = "global" }, true},
		{"user scope is valid", func(tg *Tag) { tg.Scope = TagScopeUser }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg := validTag()
			tt.mutate(&tg)
			err := tg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
