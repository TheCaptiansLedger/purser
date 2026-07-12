package domain

import "testing"

func validPerson() Person {
	return Person{
		ID:          "p1",
		Name:        "Riley Reid",
		Gender:      GenderFemale,
		MonitorMode: MonitorModeAll,
	}
}

func TestPerson_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Person)
		wantErr bool
	}{
		{"valid", func(_ *Person) {}, false},
		{"missing ID", func(p *Person) { p.ID = "" }, true},
		{"missing Name", func(p *Person) { p.Name = "" }, true},
		{"missing Gender", func(p *Person) { p.Gender = "" }, true},
		{"invalid Gender", func(p *Person) { p.Gender = "APACHE_HELICOPTER" }, true},
		{"missing MonitorMode", func(p *Person) { p.MonitorMode = "" }, true},
		{"invalid MonitorMode", func(p *Person) { p.MonitorMode = "sometimes" }, true},
		{"unknown gender is valid", func(p *Person) { p.Gender = GenderUnknown }, false},
		{"non-binary gender is valid", func(p *Person) { p.Gender = GenderNonBinary }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validPerson()
			tt.mutate(&p)
			err := p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGender_String(t *testing.T) {
	if got := GenderFemale.String(); got != "FEMALE" {
		t.Errorf("String() = %q, want %q", got, "FEMALE")
	}
}

func TestGender_Valid(t *testing.T) {
	tests := []struct {
		name string
		g    Gender
		want bool
	}{
		{"male", GenderMale, true},
		{"female", GenderFemale, true},
		{"transgender male", GenderTransgenderMale, true},
		{"transgender female", GenderTransgenderFemale, true},
		{"intersex", GenderIntersex, true},
		{"non-binary", GenderNonBinary, true},
		{"unknown", GenderUnknown, true},
		{"empty", Gender(""), false},
		{"garbage", Gender("APACHE_HELICOPTER"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.g.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}
