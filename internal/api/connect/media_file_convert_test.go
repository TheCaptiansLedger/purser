package apiconnect

import (
	"purser/internal/domain"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestMediaFileRoundTrip(t *testing.T) {
	if mediaFileToProto(nil) != nil {
		t.Fatal("mediaFileToProto(nil) did not return nil")
	}
	if mediaFileFromProto(nil) == nil {
		t.Fatal("mediaFileFromProto(nil) returned nil, want a zero-value MediaFile")
	}

	m := &domain.MediaFile{
		ID: "m1", ItemID: "i1", Path: "/media/f.mkv", Size: 1024,
		OSHash: "osh", MD5: "md5", SHA1: "sha1", Quality: "1080p",
		Resolution: "1920x1080", Codec: "h264", Container: "mkv",
		Metadata: map[string]string{"k": "v"},
	}
	got := mediaFileFromProto(mediaFileToProto(m))
	if got.ID != m.ID || got.ItemID != m.ItemID || got.Path != m.Path || got.Size != m.Size ||
		got.OSHash != m.OSHash || got.MD5 != m.MD5 || got.SHA1 != m.SHA1 || got.Quality != m.Quality ||
		got.Resolution != m.Resolution || got.Codec != m.Codec || got.Container != m.Container ||
		got.Metadata["k"] != "v" {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
}

func TestApplyMediaFileFieldMask(t *testing.T) {
	existing := &domain.MediaFile{ID: "m1", Path: "/media/original.mkv", Quality: "720p"}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyMediaFileFieldMask(existing, &v1.MediaFile{Id: "m1", Path: "/media/new.mkv"}, nil)
		if got.Path != "/media/new.mkv" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.MediaFile{
			ItemId: "i2", Path: "/p2", Size: 2048, OsHash: "h2", Md5: "m2", Sha1: "s2",
			Quality: "4k", Resolution: "3840x2160", Codec: "hevc", Container: "mp4",
			Metadata: map[string]string{"k": "v"},
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"item_id", "path", "size", "os_hash", "md5", "sha1", "quality",
			"resolution", "codec", "container", "metadata", "unrecognized",
		}}
		got := applyMediaFileFieldMask(existing, full, mask)
		if got.ItemID != "i2" || got.Path != "/p2" || got.Size != 2048 || got.OSHash != "h2" ||
			got.MD5 != "m2" || got.SHA1 != "s2" || got.Quality != "4k" || got.Resolution != "3840x2160" ||
			got.Codec != "hevc" || got.Container != "mp4" || got.Metadata["k"] != "v" {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
