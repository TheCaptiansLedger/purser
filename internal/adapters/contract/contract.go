// Package contract provides a shared test suite that every storage backend must
// pass. Each sub-contract exercises one repository interface; RunAll wires them
// together against a single BackendSuite.
//
// Usage in an adapter test file:
//
//	func TestMyBackend_Contract(t *testing.T) {
//	    contract.RunAll(t, contract.BackendSuite{
//	        LibraryEntries: mybackend.NewLibraryEntryRepo(db),
//	        ...
//	    })
//	}
package contract

import (
	"purser/internal/ports"
	"testing"
)

// BackendSuite holds every repository for one storage backend.
type BackendSuite struct {
	LibraryEntries  ports.LibraryEntryRepository
	Groups          ports.GroupRepository
	Items           ports.ItemRepository
	MediaFiles      ports.MediaFileRepository
	People          ports.PersonRepository
	Tags            ports.TagRepository
	ExternalIDs     ports.ExternalIDRepository
	Settings        ports.SettingsRepository
	Unmatched       ports.UnmatchedFileRepository
	MusicReleases   ports.MusicReleaseRepository
	MusicScanGroups ports.MusicScanGroupRepository
}

// RunAll executes the full contract suite for a backend.
// Each repository contract runs as a named sub-test so failures are easy to
// locate and individual contracts can be targeted with -run.
func RunAll(t *testing.T, s BackendSuite) {
	t.Helper()
	t.Run("Settings", func(t *testing.T) { runSettingsContract(t, s) })
	t.Run("Person", func(t *testing.T) { runPersonContract(t, s) })
	t.Run("Tag", func(t *testing.T) { runTagContract(t, s) })
	t.Run("LibraryEntry", func(t *testing.T) { runLibraryEntryContract(t, s) })
	t.Run("Group", func(t *testing.T) { runGroupContract(t, s) })
	t.Run("Item", func(t *testing.T) { runItemContract(t, s) })
	t.Run("MediaFile", func(t *testing.T) { runMediaFileContract(t, s) })
	t.Run("ExternalID", func(t *testing.T) { runExternalIDContract(t, s) })
	t.Run("UnmatchedFile", func(t *testing.T) { runUnmatchedFileContract(t, s) })
	t.Run("MusicRelease", func(t *testing.T) { runMusicReleaseContract(t, s) })
	t.Run("MusicScanGroup", func(t *testing.T) { runMusicScanGroupContract(t, s) })
}
