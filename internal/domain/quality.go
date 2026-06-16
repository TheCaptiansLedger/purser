package domain

// Quality represents a video resolution tier.
type Quality string

const (
	Quality4K   Quality = "4K"
	Quality1080 Quality = "1080p"
	Quality720  Quality = "720p"
	Quality480  Quality = "480p"
	QualitySD   Quality = "SD"
)

// QualityFromHeight maps a video pixel height to the closest Quality tier.
func QualityFromHeight(height int) Quality {
	switch {
	case height >= 2160:
		return Quality4K
	case height >= 1080:
		return Quality1080
	case height >= 720:
		return Quality720
	case height >= 480:
		return Quality480
	default:
		return QualitySD
	}
}
