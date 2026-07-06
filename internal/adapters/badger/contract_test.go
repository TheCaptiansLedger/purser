package badger_test

import (
	"purser/internal/adapters/badger"
	"purser/internal/adapters/contract"
	"testing"
)

func TestBadgerBackend_Contract(t *testing.T) {
	db := setupTestDB(t)
	contract.RunAll(t, contract.BackendSuite{
		LibraryEntries:  badger.NewLibraryEntryRepo(db),
		Groups:          badger.NewGroupRepo(db),
		Items:           badger.NewItemRepo(db),
		MediaFiles:      badger.NewMediaFileRepo(db),
		People:          badger.NewPersonRepo(db),
		Tags:            badger.NewTagRepo(db),
		ExternalIDs:     badger.NewExternalIDRepo(db),
		Settings:        badger.NewSettingsRepo(db),
		Unmatched:       badger.NewUnmatchedFileRepo(db),
		MusicReleases:   badger.NewMusicReleaseRepo(db),
		MusicScanGroups: badger.NewMusicScanGroupRepo(db),
	})
}
