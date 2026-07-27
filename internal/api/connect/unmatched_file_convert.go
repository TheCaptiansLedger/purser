package apiconnect

import (
	"purser/internal/domain"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"
)

func unmatchedFileStatusToProto(s domain.UnmatchedFileStatus) pipelinev1.UnmatchedFileStatus {
	switch s {
	case domain.UnmatchedFileStatusPending:
		return pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_PENDING
	case domain.UnmatchedFileStatusMatched:
		return pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_MATCHED
	case domain.UnmatchedFileStatusDismissed:
		return pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_DISMISSED
	default:
		return pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_UNSPECIFIED
	}
}

// protoToUnmatchedFileStatus is the reverse of unmatchedFileStatusToProto,
// used to convert a ListUnmatchedFiles request's status filter.
// UNMATCHED_FILE_STATUS_UNSPECIFIED (and any unrecognized value) maps to
// "" — no filter on that dimension.
func protoToUnmatchedFileStatus(s pipelinev1.UnmatchedFileStatus) domain.UnmatchedFileStatus {
	switch s {
	case pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_PENDING:
		return domain.UnmatchedFileStatusPending
	case pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_MATCHED:
		return domain.UnmatchedFileStatusMatched
	case pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_DISMISSED:
		return domain.UnmatchedFileStatusDismissed
	default:
		return ""
	}
}

func unmatchedFileToProto(u *domain.UnmatchedFile) *pipelinev1.UnmatchedFile {
	if u == nil {
		return nil
	}
	return &pipelinev1.UnmatchedFile{
		Id:           u.ID,
		Path:         u.Path,
		ContentType:  string(u.ContentType),
		Size:         u.Size,
		OsHash:       u.OSHash,
		Md5:          u.MD5,
		Sha1:         u.SHA1,
		Sha512:       u.SHA512,
		DiscoveredAt: timestampToProto(u.DiscoveredAt),
		Status:       unmatchedFileStatusToProto(u.Status),
		GroupKey:     u.GroupKey,
		DiscNumber:   toInt32(u.DiscNumber),
		TrackNumber:  u.TrackNumber,
		Fingerprint:  fingerprintToProto(u.Fingerprint),
		Candidates:   matchCandidatesToProto(u.Candidates),
	}
}

func matchTierToProto(t domain.MatchTier) pipelinev1.MatchTier {
	switch t {
	case domain.MatchTierDirectID:
		return pipelinev1.MatchTier_MATCH_TIER_DIRECT_ID
	case domain.MatchTierUniqueID:
		return pipelinev1.MatchTier_MATCH_TIER_UNIQUE_ID
	case domain.MatchTierFuzzy:
		return pipelinev1.MatchTier_MATCH_TIER_FUZZY
	case domain.MatchTierAcoustic:
		return pipelinev1.MatchTier_MATCH_TIER_ACOUSTIC
	default:
		return pipelinev1.MatchTier_MATCH_TIER_UNSPECIFIED
	}
}

func fingerprintToProto(f *domain.Fingerprint) *pipelinev1.Fingerprint {
	if f == nil {
		return nil
	}
	return &pipelinev1.Fingerprint{
		Tags:     f.Tags,
		Metadata: metadataToProto(f.Metadata),
	}
}

func matchCandidatesToProto(cs []domain.MatchCandidate) []*pipelinev1.MatchCandidate {
	if cs == nil {
		return nil
	}
	out := make([]*pipelinev1.MatchCandidate, 0, len(cs))
	for _, c := range cs {
		out = append(out, &pipelinev1.MatchCandidate{
			ExternalRef: c.ExternalRef,
			Title:       c.Title,
			Score:       c.Score,
			Tier:        matchTierToProto(c.Tier),
			Signals:     c.Signals,
			Metadata:    metadataToProto(c.Metadata),
		})
	}
	return out
}
