package domain

// MergeExternalItems merges an ordered slice of provider results into a single
// ExternalItem. results must be ordered by provider priority (index 0 = primary).
// Returns nil for an empty slice.
func MergeExternalItems(results []SourcedExternalItem) *ExternalItem {
	if len(results) == 0 {
		return nil
	}

	merged := &ExternalItem{}
	seenURLs := make(map[string]bool)
	seenTags := make(map[string]bool)
	seenGenres := make(map[string]bool)

	for _, r := range results {
		fillScalars(merged, r.Item)

		// image union: stamp each image with its provider source, deduplicate by URL
		for _, img := range r.Item.Images {
			if !seenURLs[img.URL] {
				seenURLs[img.URL] = true
				img.Source = r.Source
				merged.Images = append(merged.Images, img)
			}
		}

		// tag union: stable deduplicated order
		for _, tag := range r.Item.Tags {
			if !seenTags[tag] {
				seenTags[tag] = true
				merged.Tags = append(merged.Tags, tag)
			}
		}

		// genre union: stable deduplicated order
		for _, genre := range r.Item.Genres {
			if !seenGenres[genre] {
				seenGenres[genre] = true
				merged.Genres = append(merged.Genres, genre)
			}
		}

		// map merge: lower-index (higher priority) source wins on key collision
		for k, v := range r.Item.ExternalIDs {
			if merged.ExternalIDs == nil {
				merged.ExternalIDs = make(map[string]string)
			}
			if _, exists := merged.ExternalIDs[k]; !exists {
				merged.ExternalIDs[k] = v
			}
		}
	}

	return merged
}

// fillScalars copies each scalar field from src into dst only when dst's field
// is the zero value. This gives primary-wins semantics across the provider list.
func fillScalars(dst, src *ExternalItem) {
	if dst.Source == "" {
		dst.Source = src.Source
	}
	if dst.ExternalID == "" {
		dst.ExternalID = src.ExternalID
	}
	if dst.ContentType == "" {
		dst.ContentType = src.ContentType
	}
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Overview == "" {
		dst.Overview = src.Overview
	}
	if dst.Year == 0 {
		dst.Year = src.Year
	}
	if dst.RuntimeSecs == 0 {
		dst.RuntimeSecs = src.RuntimeSecs
	}
	if dst.Date.IsZero() {
		dst.Date = src.Date
	}
	if dst.ImageURL == "" {
		dst.ImageURL = src.ImageURL
	}
	if dst.Studio == nil {
		dst.Studio = src.Studio
	}
	if dst.People == nil {
		dst.People = src.People
	}
}
