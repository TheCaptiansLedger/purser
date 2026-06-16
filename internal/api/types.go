package api

import "time"

// Shared response sub-types used across multiple handler files.

type externalIDResponse struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

type tagResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
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

type mediaFileResponse struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	OSHash     string    `json:"osHash"`
	Quality    string    `json:"quality"`
	Resolution string    `json:"resolution"`
	Codec      string    `json:"codec"`
	Container  string    `json:"container"`
	AddedAt    time.Time `json:"addedAt"`
}
