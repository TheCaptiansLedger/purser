package domain

import "testing"

func validEntryPerson() EntryPerson {
	return EntryPerson{
		LibraryEntryID: "le1",
		PersonID:       "p1",
		Role:           "member",
	}
}

func TestEntryPerson_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*EntryPerson)
		wantErr bool
	}{
		{"valid", func(_ *EntryPerson) {}, false},
		{"valid with CreditedAs and Character", func(e *EntryPerson) {
			e.CreditedAs = "Anikka"
			e.Character = "Walter White"
		}, false},
		{"missing LibraryEntryID", func(e *EntryPerson) { e.LibraryEntryID = "" }, true},
		{"missing PersonID", func(e *EntryPerson) { e.PersonID = "" }, true},
		{"missing Role", func(e *EntryPerson) { e.Role = "" }, true},
		{"any non-empty Role is valid (open enum)", func(e *EntryPerson) {
			e.Role = "chief_gaffer"
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEntryPerson()
			tt.mutate(&e)
			err := e.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func validItemPerson() ItemPerson {
	return ItemPerson{
		ItemID:   "i1",
		PersonID: "p1",
		Role:     "featured_artist",
	}
}

func TestItemPerson_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ItemPerson)
		wantErr bool
	}{
		{"valid", func(_ *ItemPerson) {}, false},
		{"valid with CreditedAs and Character", func(i *ItemPerson) {
			i.CreditedAs = "Anikka"
			i.Character = "Steven Gomez"
		}, false},
		{"missing ItemID", func(i *ItemPerson) { i.ItemID = "" }, true},
		{"missing PersonID", func(i *ItemPerson) { i.PersonID = "" }, true},
		{"missing Role", func(i *ItemPerson) { i.Role = "" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := validItemPerson()
			tt.mutate(&i)
			err := i.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
