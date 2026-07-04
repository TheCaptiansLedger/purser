package db

import (
	"purser/internal/adapters/contract"
	"testing"
)

func TestSQLBackend_Contract(t *testing.T) {
	database := setupTestDB(t)
	contract.RunAll(t, contract.BackendSuite{
		LibraryEntries: NewLibraryEntryRepo(database),
		Groups:         NewGroupRepo(database),
		Items:          NewItemRepo(database),
		MediaFiles:     NewMediaFileRepo(database),
		People:         NewPersonRepo(database),
		Tags:           NewTagRepo(database),
		ExternalIDs:    NewExternalIDRepo(database),
		Settings:       NewSettingsRepo(database),
		Unmatched:      NewUnmatchedFileRepo(database),
	})
}
