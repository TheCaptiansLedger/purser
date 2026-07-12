package apiconnect

import (
	"purser/internal/domain"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestGenderRoundTrip(t *testing.T) {
	cases := []struct {
		domainVal domain.Gender
		protoVal  v1.Gender
	}{
		{domain.GenderMale, v1.Gender_GENDER_MALE},
		{domain.GenderFemale, v1.Gender_GENDER_FEMALE},
		{domain.GenderTransgenderMale, v1.Gender_GENDER_TRANSGENDER_MALE},
		{domain.GenderTransgenderFemale, v1.Gender_GENDER_TRANSGENDER_FEMALE},
		{domain.GenderIntersex, v1.Gender_GENDER_INTERSEX},
		{domain.GenderNonBinary, v1.Gender_GENDER_NON_BINARY},
		{domain.GenderUnknown, v1.Gender_GENDER_UNKNOWN},
	}
	for _, c := range cases {
		if got := genderToProto(c.domainVal); got != c.protoVal {
			t.Errorf("genderToProto(%v) = %v, want %v", c.domainVal, got, c.protoVal)
		}
		if got := genderFromProto(c.protoVal); got != c.domainVal {
			t.Errorf("genderFromProto(%v) = %v, want %v", c.protoVal, got, c.domainVal)
		}
	}

	if got := genderFromProto(v1.Gender_GENDER_UNSPECIFIED); got != domain.Gender("") {
		t.Errorf("genderFromProto(GENDER_UNSPECIFIED) = %q, want zero value", got)
	}
	if got := genderToProto(domain.Gender("bogus")); got != v1.Gender_GENDER_UNSPECIFIED {
		t.Errorf("genderToProto(bogus) = %v, want GENDER_UNSPECIFIED", got)
	}
}

func TestMonitorModeRoundTrip(t *testing.T) {
	cases := []struct {
		domainVal domain.MonitorMode
		protoVal  v1.MonitorMode
	}{
		{domain.MonitorModeAll, v1.MonitorMode_MONITOR_MODE_ALL},
		{domain.MonitorModeFuture, v1.MonitorMode_MONITOR_MODE_FUTURE},
		{domain.MonitorModeNone, v1.MonitorMode_MONITOR_MODE_NONE},
		{domain.MonitorModeLatest, v1.MonitorMode_MONITOR_MODE_LATEST},
	}
	for _, c := range cases {
		if got := monitorModeToProto(c.domainVal); got != c.protoVal {
			t.Errorf("monitorModeToProto(%v) = %v, want %v", c.domainVal, got, c.protoVal)
		}
		if got := monitorModeFromProto(c.protoVal); got != c.domainVal {
			t.Errorf("monitorModeFromProto(%v) = %v, want %v", c.protoVal, got, c.domainVal)
		}
	}

	if got := monitorModeFromProto(v1.MonitorMode_MONITOR_MODE_UNSPECIFIED); got != domain.MonitorMode("") {
		t.Errorf("monitorModeFromProto(MONITOR_MODE_UNSPECIFIED) = %q, want zero value", got)
	}
	if got := monitorModeToProto(domain.MonitorMode("bogus")); got != v1.MonitorMode_MONITOR_MODE_UNSPECIFIED {
		t.Errorf("monitorModeToProto(bogus) = %v, want MONITOR_MODE_UNSPECIFIED", got)
	}
}

func TestPersonRoundTrip(t *testing.T) {
	t.Run("nil in, nil/zero out", func(t *testing.T) {
		if personToProto(nil) != nil {
			t.Fatal("personToProto(nil) did not return nil")
		}
		if personFromProto(nil) == nil {
			t.Fatal("personFromProto(nil) returned nil, want a zero-value Person")
		}
	})

	t.Run("full round trip including optional dates", func(t *testing.T) {
		birth := time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC)
		death := time.Date(2020, 3, 4, 0, 0, 0, 0, time.UTC)
		added := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)

		p := &domain.Person{
			ID:          "p1",
			Name:        "Full Person",
			SortName:    "Person, Full",
			Aliases:     []string{"alias1", "alias2"},
			Gender:      domain.GenderNonBinary,
			Pronouns:    "they/them",
			BirthDate:   &birth,
			DeathDate:   &death,
			Nationality: "US",
			Overview:    "An overview",
			Monitored:   true,
			MonitorMode: domain.MonitorModeAll,
			AddedAt:     added,
			UpdatedAt:   added,
		}

		got := personFromProto(personToProto(p))

		if got.ID != p.ID || got.Name != p.Name || got.SortName != p.SortName {
			t.Fatalf("round trip changed identity fields: got %+v", got)
		}
		if len(got.Aliases) != 2 || got.Aliases[0] != "alias1" {
			t.Fatalf("round trip changed Aliases: got %v", got.Aliases)
		}
		if got.Gender != p.Gender || got.Pronouns != p.Pronouns || got.MonitorMode != p.MonitorMode {
			t.Fatalf("round trip changed Gender/Pronouns/MonitorMode: got %+v", got)
		}
		if got.BirthDate == nil || !got.BirthDate.Equal(birth) {
			t.Fatalf("round trip changed BirthDate: got %v", got.BirthDate)
		}
		if got.DeathDate == nil || !got.DeathDate.Equal(death) {
			t.Fatalf("round trip changed DeathDate: got %v", got.DeathDate)
		}
		if !got.AddedAt.Equal(added) || !got.UpdatedAt.Equal(added) {
			t.Fatalf("round trip changed AddedAt/UpdatedAt: got %v / %v", got.AddedAt, got.UpdatedAt)
		}
	})

	t.Run("nil optional dates stay nil", func(t *testing.T) {
		p := &domain.Person{ID: "p1", Name: "No Dates"}
		got := personFromProto(personToProto(p))
		if got.BirthDate != nil || got.DeathDate != nil {
			t.Fatalf("round trip invented a date: BirthDate=%v DeathDate=%v", got.BirthDate, got.DeathDate)
		}
	})
}

func TestApplyPersonFieldMask(t *testing.T) {
	existing := &domain.Person{
		ID: "p1", Name: "Original", Overview: "Original Overview",
		Gender: domain.GenderUnknown, MonitorMode: domain.MonitorModeNone,
	}
	incoming := &v1.Person{
		Id: "p1", Name: "New Name", Overview: "New Overview",
		Gender: v1.Gender_GENDER_MALE, MonitorMode: v1.MonitorMode_MONITOR_MODE_ALL,
	}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyPersonFieldMask(existing, incoming, nil)
		if got.Name != "New Name" || got.Overview != "New Overview" || got.Gender != domain.GenderMale {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
		if got.ID != "p1" {
			t.Fatalf("nil mask changed ID: got %q", got.ID)
		}
	})

	t.Run("empty-paths mask replaces every mutable field", func(t *testing.T) {
		got := applyPersonFieldMask(existing, incoming, &fieldmaskpb.FieldMask{})
		if got.Name != "New Name" {
			t.Fatalf("empty mask did not fully replace: got %+v", got)
		}
	})

	t.Run("named paths apply only those fields", func(t *testing.T) {
		mask := &fieldmaskpb.FieldMask{Paths: []string{"name"}}
		got := applyPersonFieldMask(existing, incoming, mask)
		if got.Name != "New Name" {
			t.Fatalf("masked Update did not apply Name: got %q", got.Name)
		}
		if got.Overview != "Original Overview" {
			t.Fatalf("masked Update touched Overview: got %q", got.Overview)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.Person{
			Name: "N", SortName: "S", Aliases: []string{"a"}, Gender: v1.Gender_GENDER_FEMALE,
			Pronouns: "she/her", Nationality: "CA", Overview: "O", Monitored: true,
			MonitorMode: v1.MonitorMode_MONITOR_MODE_LATEST,
			BirthDate:   nil, DeathDate: nil,
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"name", "sort_name", "aliases", "gender", "pronouns",
			"birth_date", "death_date", "nationality", "overview",
			"monitored", "monitor_mode", "unrecognized_path",
		}}
		got := applyPersonFieldMask(existing, full, mask)
		if got.Name != "N" || got.SortName != "S" || len(got.Aliases) != 1 || got.Gender != domain.GenderFemale ||
			got.Pronouns != "she/her" || got.Nationality != "CA" || got.Overview != "O" || !got.Monitored ||
			got.MonitorMode != domain.MonitorModeLatest {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
