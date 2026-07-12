package afterdark_test

import (
	"errors"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"testing"
)

func validPerformerProfile() *afterdark.PerformerProfile {
	return &afterdark.PerformerProfile{PersonID: "p1"}
}

func TestPerformerProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*afterdark.PerformerProfile)
		wantErr bool
	}{
		{
			name:    "valid profile with only PersonID set",
			mutate:  func(*afterdark.PerformerProfile) {},
			wantErr: false,
		},
		{
			name: "valid profile with every field populated",
			mutate: func(p *afterdark.PerformerProfile) {
				p.CupSize = "34"
				p.BandSize = "C"
				p.BreastType = "natural"
				p.Tattoos = []afterdark.BodyMark{{Location: "left arm", Description: "rose"}}
				p.Piercings = []afterdark.BodyMark{{Location: "navel", Description: "ring"}}
				p.CareerStartYear = 2015
				p.CareerEndYear = 2020
			},
			wantErr: false,
		},
		{
			name:    "missing PersonID is invalid",
			mutate:  func(p *afterdark.PerformerProfile) { p.PersonID = "" },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validPerformerProfile()
			tt.mutate(p)

			err := p.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestPerformerProfile_Validate_ReturnsDomainValidationError(t *testing.T) {
	p := validPerformerProfile()
	p.PersonID = ""

	var verr *domain.ValidationError
	err := p.Validate()
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() returned %v, want *domain.ValidationError", err)
	}
	if len(verr.Errors) != 1 || verr.Errors[0].Field != "PersonID" {
		t.Fatalf("Validate() returned %+v, want a single PersonID field error", verr)
	}
}
