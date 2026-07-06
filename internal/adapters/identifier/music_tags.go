package identifier

import (
	"context"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"strconv"
	"time"
)

// ExtractMusicTagSummary derives consensus tag values from all fingerprinted files in the group.
// Files without a Fingerprint are skipped. Tracks are ordered by (disc_number, track_number).
func ExtractMusicTagSummary(ctx context.Context, group ports.ScannedFileGroup) domain.MusicTagSummary {
	type trackData struct {
		discNum    int
		number     int
		title      string
		isrc       string
		durationMS int
	}

	albumArtistVals := make([]string, 0, len(group.Files))
	albumTitleVals := make([]string, 0, len(group.Files))
	yearVals := make([]string, 0, len(group.Files))
	barcodeVals := make([]string, 0, len(group.Files))
	labelVals := make([]string, 0, len(group.Files))
	catalogVals := make([]string, 0, len(group.Files))
	totalTracksVals := make([]string, 0, len(group.Files))
	totalDiscsVals := make([]string, 0, len(group.Files))
	mbzReleaseVals := make([]string, 0, len(group.Files))

	tracks := make([]trackData, 0, len(group.Files))

	for _, f := range group.Files {
		if f.Fingerprint == nil {
			continue
		}
		tags := f.Fingerprint.EmbeddedTags

		albumArtistVals = append(albumArtistVals, tags["album_artist"])
		albumTitleVals = append(albumTitleVals, tags["album"])
		yearVals = append(yearVals, tags["date"]) // fingerprinter stores year under "date"
		barcodeVals = append(barcodeVals, tags["barcode"])
		labelVals = append(labelVals, tags["label"])
		catalogVals = append(catalogVals, tags["catalog_number"])
		totalTracksVals = append(totalTracksVals, tags["track_total"])
		totalDiscsVals = append(totalDiscsVals, tags["disc_total"])
		mbzReleaseVals = append(mbzReleaseVals, tags["musicbrainz_album_id"])

		discNum, _ := strconv.Atoi(tags["disc_number"])
		trackNum, _ := strconv.Atoi(tags["track_number"])
		durMS, _ := strconv.Atoi(tags["duration_ms"])
		tracks = append(tracks, trackData{
			discNum:    discNum,
			number:     trackNum,
			title:      tags["title"],
			isrc:       tags["isrc"],
			durationMS: durMS,
		})
	}

	// Sort by (disc_number, track_number) so multi-disc albums preserve global order.
	sort.SliceStable(tracks, func(i, j int) bool {
		if tracks[i].discNum != tracks[j].discNum {
			return tracks[i].discNum < tracks[j].discNum
		}
		return tracks[i].number < tracks[j].number
	})

	albumArtist := consensusWarn(ctx, group.RootPath, "album_artist", albumArtistVals)
	albumTitle := consensusWarn(ctx, group.RootPath, "album", albumTitleVals)
	yearStr := consensusWarn(ctx, group.RootPath, "date", yearVals)
	barcode := consensusWarn(ctx, group.RootPath, "barcode", barcodeVals)
	label := consensusWarn(ctx, group.RootPath, "label", labelVals)
	catalog := consensusWarn(ctx, group.RootPath, "catalog_number", catalogVals)
	trackTotalStr := consensusWarn(ctx, group.RootPath, "track_total", totalTracksVals)
	discTotalStr := consensusWarn(ctx, group.RootPath, "disc_total", totalDiscsVals)
	mbzRelease := consensusWarn(ctx, group.RootPath, "musicbrainz_album_id", mbzReleaseVals)

	year, _ := strconv.Atoi(yearStr)
	totalTracks, _ := strconv.Atoi(trackTotalStr)
	totalDiscs, _ := strconv.Atoi(discTotalStr)

	// TotalDiscs fallback: count distinct disc_number tag values when disc_total is absent.
	if totalDiscs == 0 {
		discNums := make(map[string]struct{})
		for _, f := range group.Files {
			if f.Fingerprint == nil {
				continue
			}
			if dn := f.Fingerprint.EmbeddedTags["disc_number"]; dn != "" {
				discNums[dn] = struct{}{}
			}
		}
		if len(discNums) > 0 {
			totalDiscs = len(discNums)
		} else {
			totalDiscs = 1
		}
	}

	trackTitles := make([]string, len(tracks))
	trackDurations := make([]time.Duration, len(tracks))
	isrcs := make([]string, len(tracks))
	for i, tr := range tracks {
		trackTitles[i] = tr.title
		trackDurations[i] = time.Duration(tr.durationMS) * time.Millisecond
		isrcs[i] = tr.isrc
	}

	hasISRCs := false
	for _, isrc := range isrcs {
		if isrc != "" {
			hasISRCs = true
			break
		}
	}

	slog.InfoContext(ctx, "music tag summary",
		"root", group.RootPath,
		"album_artist", albumArtist,
		"album", albumTitle,
		"year", year,
		"barcode", barcode,
		"total_tracks", totalTracks,
		"total_discs", totalDiscs,
		"mbz_release_id", mbzRelease,
		"has_isrcs", hasISRCs,
	)

	return domain.MusicTagSummary{
		AlbumArtist:    albumArtist,
		AlbumTitle:     albumTitle,
		Year:           year,
		Barcode:        barcode,
		Label:          label,
		CatalogNumber:  catalog,
		TotalTracks:    totalTracks,
		TotalDiscs:     totalDiscs,
		MBZReleaseID:   mbzRelease,
		TrackTitles:    trackTitles,
		TrackDurations: trackDurations,
		ISRCs:          isrcs,
	}
}

// consensusWarn returns the majority value from vals. When multiple values tie
// for the top count, the first non-empty value in original order is returned and a WARN is logged.
func consensusWarn(ctx context.Context, root, key string, vals []string) string {
	counts := make(map[string]int)
	for _, v := range vals {
		if v != "" {
			counts[v]++
		}
	}
	if len(counts) == 0 {
		return ""
	}

	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	winnerCount := 0
	for _, c := range counts {
		if c == maxCount {
			winnerCount++
		}
	}
	if winnerCount == 1 {
		for v, c := range counts {
			if c == maxCount {
				return v
			}
		}
	}

	// Tie: warn and return first non-empty value preserving original input order.
	var allVals []string
	for _, v := range vals {
		if v != "" {
			allVals = append(allVals, v)
		}
	}
	slog.WarnContext(ctx, "tag consensus failed", "root", root, "key", key, "values", allVals)
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
