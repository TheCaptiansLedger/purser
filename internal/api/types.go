package api

import "time"

// Shared response sub-types used across multiple handler files.

type externalIDResponse struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

type tagResponse struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

type personRefResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SortName string `json:"sortName"`
	ImageURL string `json:"imageUrl,omitempty"`
}

type itemPersonResponse struct {
	PersonID string             `json:"personId"`
	Person   *personRefResponse `json:"person,omitempty"`
	Role     string             `json:"role"`
}

type entryPersonResponse struct {
	PersonID  string             `json:"personId"`
	Person    *personRefResponse `json:"person,omitempty"`
	Role      string             `json:"role"`
	StartDate string             `json:"startDate,omitempty"`
	EndDate   string             `json:"endDate,omitempty"`
}

type mediaFileResponse struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	OSHash     string    `json:"osHash"`
	MD5        string    `json:"md5,omitempty"`
	Quality    string    `json:"quality"`
	Resolution string    `json:"resolution"`
	Codec      string    `json:"codec"`
	Container  string    `json:"container"`
	AddedAt    time.Time `json:"addedAt"`
}

type musicConfidenceSignalsResponse struct {
	Barcode       float64 `json:"barcode"`
	ISRC          float64 `json:"isrc"`
	RGNameFuzzy   float64 `json:"rgNameFuzzy"`
	TrackCount    float64 `json:"trackCount"`
	TrackTitleSet float64 `json:"trackTitleSet"`
	Duration      float64 `json:"duration"`
	AcoustID      float64 `json:"acoustid"`
}

type musicReleaseCandidateResponse struct {
	ArtistMBID         string                         `json:"artistMbid,omitempty"`
	ArtistName         string                         `json:"artistName"`
	ReleaseGroupMBID   string                         `json:"releaseGroupMbid,omitempty"`
	ReleaseGroupTitle  string                         `json:"releaseGroupTitle"`
	ReleaseGroupType   string                         `json:"releaseGroupType,omitempty"`
	ReleaseMBID        string                         `json:"releaseMbid,omitempty"`
	ReleaseTitle       string                         `json:"releaseTitle"`
	ReleaseDate        string                         `json:"releaseDate,omitempty"`
	ReleaseLabel       string                         `json:"releaseLabel,omitempty"`
	ReleaseCountry     string                         `json:"releaseCountry,omitempty"`
	ReleaseBarcode     string                         `json:"releaseBarcode,omitempty"`
	ReleaseFormat      string                         `json:"releaseFormat,omitempty"`
	ReleaseMediumCount int                            `json:"releaseMediumCount"`
	ReleaseTrackCount  int                            `json:"releaseTrackCount"`
	OverallConfidence  float64                        `json:"overallConfidence"`
	Signals            musicConfidenceSignalsResponse `json:"signals"`
}

type musicScanGroupResponse struct {
	ID           string                          `json:"id"`
	FolderPath   string                          `json:"folderPath"`
	TotalTracks  int                             `json:"totalTracks"`
	TotalDiscs   int                             `json:"totalDiscs"`
	Status       string                          `json:"status"`
	Candidates   []musicReleaseCandidateResponse `json:"candidates"`
	DiscoveredAt time.Time                       `json:"discoveredAt"`
}

type musicReleaseResponse struct {
	ID             string               `json:"id"`
	GroupID        string               `json:"groupId"`
	LibraryEntryID string               `json:"libraryEntryId"`
	Title          string               `json:"title"`
	Country        string               `json:"country"`
	Date           string               `json:"date"`
	Label          string               `json:"label"`
	CatalogNumber  string               `json:"catalogNumber"`
	Barcode        string               `json:"barcode"`
	Format         string               `json:"format"`
	MediumCount    int                  `json:"mediumCount"`
	TrackCount     int                  `json:"trackCount"`
	IsDefault      bool                 `json:"isDefault"`
	Monitored      bool                 `json:"monitored"`
	Status         string               `json:"status"`
	ExternalIDs    []externalIDResponse `json:"externalIds"`
	CoverURL       string               `json:"coverUrl,omitempty"`
	AddedAt        time.Time            `json:"addedAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}
