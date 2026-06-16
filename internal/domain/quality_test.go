package domain

import "testing"

func TestQualityFromHeight(t *testing.T) {
	cases := []struct {
		height int
		want   Quality
	}{
		{2161, Quality4K},
		{2160, Quality4K},
		{1440, Quality1080},
		{1080, Quality1080},
		{1079, Quality720},
		{720, Quality720},
		{719, Quality480},
		{480, Quality480},
		{479, QualitySD},
		{360, QualitySD},
		{0, QualitySD},
	}
	for _, tc := range cases {
		if got := QualityFromHeight(tc.height); got != tc.want {
			t.Errorf("QualityFromHeight(%d) = %q, want %q", tc.height, got, tc.want)
		}
	}
}
