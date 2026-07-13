package apiconnect

import (
	"purser/internal/domain"
	"purser/internal/domain/afterdark"

	v1 "purser/gen/go/purser/domain/v1"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// itemsToProto converts a slice of domain.Item — used by BrowseHandler's
// scene-listing RPCs, which (unlike ItemHandler's own ListItems) don't
// build this conversion inline since they're not itemToProto's usual
// single-entity-handler call site.
func itemsToProto(items []*domain.Item) []*v1.Item {
	pb := make([]*v1.Item, 0, len(items))
	for _, i := range items {
		pb = append(pb, itemToProto(i))
	}
	return pb
}

func performerViewToProto(v *afterdark.PerformerView) *afterdarkv1.PerformerView {
	if v == nil {
		return nil
	}
	return &afterdarkv1.PerformerView{
		Person:           personToProto(v.Person),
		PerformerProfile: performerProfileToProto(v.Profile),
	}
}

func performerViewsToProto(views []*afterdark.PerformerView) []*afterdarkv1.PerformerView {
	pb := make([]*afterdarkv1.PerformerView, 0, len(views))
	for _, v := range views {
		pb = append(pb, performerViewToProto(v))
	}
	return pb
}
