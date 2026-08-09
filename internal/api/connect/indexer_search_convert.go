package apiconnect

import (
	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
	"purser/internal/ports"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// protocolToProto maps ports.Protocol to its wire enum. An unrecognized
// value (never expected from a well-behaved IndexerSearcher adapter) maps
// to PROTOCOL_UNSPECIFIED rather than panicking.
func protocolToProto(p ports.Protocol) acquisitionv1.Protocol {
	switch p {
	case ports.ProtocolTorrent:
		return acquisitionv1.Protocol_PROTOCOL_TORRENT
	case ports.ProtocolUsenet:
		return acquisitionv1.Protocol_PROTOCOL_USENET
	default:
		return acquisitionv1.Protocol_PROTOCOL_UNSPECIFIED
	}
}

// categoryToProto maps ports.Category (internal/adapters/prowlarr's own
// DTO) to its wire shape.
func categoryToProto(c ports.Category) *acquisitionv1.Category {
	return &acquisitionv1.Category{
		Id:   int32(c.ID), //nolint:gosec // an indexer's own category id is never remotely close to overflowing int32
		Name: c.Name,
	}
}

// indexerReleaseToProto maps ports.IndexerRelease to its wire shape.
// PublishDate is always set — internal/adapters/prowlarr.Client's own
// parsePublishDate degrades a malformed/absent value to the zero
// time.Time rather than leaving it unset, so this always calls
// timestamppb.New unconditionally, the same convention job_convert.go's
// jobToProto uses for its own non-pointer time.Time fields.
func indexerReleaseToProto(r ports.IndexerRelease) *acquisitionv1.IndexerRelease {
	cats := make([]*acquisitionv1.Category, len(r.Categories))
	for i, c := range r.Categories {
		cats[i] = categoryToProto(c)
	}

	return &acquisitionv1.IndexerRelease{
		Guid:        r.GUID,
		Title:       r.Title,
		IndexerName: r.IndexerName,
		Size:        r.Size,
		Protocol:    protocolToProto(r.Protocol),
		PublishDate: timestamppb.New(r.PublishDate),
		Seeders:     int32(r.Seeders),  //nolint:gosec // an indexer's seeder count is never remotely close to overflowing int32
		Leechers:    int32(r.Leechers), //nolint:gosec // same — leecher count
		DownloadUrl: r.DownloadURL,
		MagnetUrl:   r.MagnetURL,
		InfoUrl:     r.InfoURL,
		InfoHash:    r.InfoHash,
		Categories:  cats,
	}
}
