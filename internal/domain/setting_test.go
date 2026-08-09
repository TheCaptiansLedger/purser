package domain

import "testing"

func validSetting() Setting {
	return Setting{
		Key:   "pipeline.confidence_threshold",
		Value: `0.75`,
	}
}

func TestSetting_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Setting)
		wantErr bool
	}{
		{"valid", func(_ *Setting) {}, false},
		{"missing Key", func(s *Setting) { s.Key = "" }, true},
		{"empty Value is valid", func(s *Setting) { s.Value = "" }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSetting()
			tt.mutate(&s)
			err := s.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
