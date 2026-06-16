package domain

import "time"

// MediaFile represents a media file present on disk, linked to an Item.
type MediaFile struct {
	ID         string
	ItemID     string
	Path       string
	Size       int64
	OSHash     string
	MD5        string
	Quality    Quality
	Resolution string // e.g. "1920x1080"
	Codec      string
	Container  string
	AddedAt    time.Time
}
