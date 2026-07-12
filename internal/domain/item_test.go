package domain

import "testing"

func validItem() Item {
	return Item{
		ID:             "i1",
		ContentType:    ContentTypeMusic,
		LibraryEntryID: "le1",
		Title:          "Dreams",
		Status:         ItemStatusWanted,
	}
}

func TestItem_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Item)
		wantErr bool
	}{
		{"valid", func(_ *Item) {}, false},
		{"missing ID", func(i *Item) { i.ID = "" }, true},
		{"missing ContentType", func(i *Item) { i.ContentType = "" }, true},
		{"any non-empty ContentType is valid (open enum)", func(i *Item) {
			i.ContentType = ContentType("comics")
		}, false},
		{"missing LibraryEntryID", func(i *Item) { i.LibraryEntryID = "" }, true},
		{"missing Title", func(i *Item) { i.Title = "" }, true},
		{"missing Status", func(i *Item) { i.Status = "" }, true},
		{"invalid Status", func(i *Item) { i.Status = "vibing" }, true},
		{"imported status is valid", func(i *Item) { i.Status = ItemStatusImported }, false},
		{"no GroupID is valid (nullable)", func(i *Item) { i.GroupID = "" }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := validItem()
			tt.mutate(&i)
			err := i.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
