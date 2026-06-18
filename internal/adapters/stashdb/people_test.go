package stashdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"testing"
)

const searchPeopleFixture = `{
  "data": {
    "queryPerformers": {
      "performers": [
        {
          "id": "perf-001",
          "name": "Jane Example",
          "aliases": ["Jane E"],
          "images": [{"url": "https://example.com/jane.jpg"}],
          "birthdate": {"date": "1995-03-15"},
          "height": 165,
          "hair_color": "Brunette",
          "eye_color": "Brown",
          "gender": "FEMALE",
          "ethnicity": "Caucasian",
          "country": "US",
          "breast_type": "NATURAL",
          "career_start_year": 2015,
          "career_end_year": 2022,
          "disambiguation": "the performer",
          "tattoos": [{"location": "arm", "description": "rose"}],
          "piercings": [],
          "measurements": {"cup_size": "C", "band_size": 32, "waist": 24, "hip": 36}
        }
      ]
    }
  }
}`

const searchPeopleNoMetaFixture = `{
  "data": {
    "queryPerformers": {
      "performers": [
        {
          "id": "perf-003",
          "name": "No Meta",
          "aliases": [],
          "images": []
        }
      ]
    }
  }
}`

func newPeopleTestAdapter(srv *httptest.Server) *stashdb.Adapter {
	return stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "test-key"})
}

func TestSearchPeople_AllFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchPeopleFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	people, err := newPeopleTestAdapter(srv).SearchPeople(context.Background(), "Jane", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("expected 1 person, got %d", len(people))
	}

	p := people[0]
	if p.ExternalID != "perf-001" {
		t.Errorf("ExternalID: want perf-001, got %s", p.ExternalID)
	}
	if p.Name != "Jane Example" {
		t.Errorf("Name: want Jane Example, got %s", p.Name)
	}
	if p.ImageURL != "https://example.com/jane.jpg" {
		t.Errorf("ImageURL: want https://example.com/jane.jpg, got %s", p.ImageURL)
	}
	if len(p.Aliases) != 1 || p.Aliases[0] != "Jane E" {
		t.Errorf("Aliases: want [Jane E], got %v", p.Aliases)
	}

	meta := p.Metadata
	cases := []struct {
		key  string
		want any
	}{
		{"birthdate", "1995-03-15"},
		{"height", "165 cm"},
		{"hair_color", "brunette"},
		{"eye_color", "brown"},
		{"gender", "female"},
		{"ethnicity", "Caucasian"},
		{"nationality", "US"},
		{"breast_type", "Natural"},
		{"career_start", "2015"},
		{"career_end", "2022"},
		{"disambiguation", "the performer"},
		{"cup_size", "C"},
		{"measurements", "32-24-36"},
		{"tattoos", "arm: rose"},
	}
	for _, tc := range cases {
		got, ok := meta[tc.key]
		if !ok {
			t.Errorf("metadata[%q]: missing", tc.key)
			continue
		}
		if got != tc.want {
			t.Errorf("metadata[%q]: want %v (%T), got %v (%T)", tc.key, tc.want, tc.want, got, got)
		}
	}
}

func TestSearchPeople_GenderNormalization(t *testing.T) {
	cases := []struct {
		name       string
		fixture    string
		wantGender string
	}{
		{"FEMALE", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"FEMALE","aliases":[],"images":[]}]}}}`, "female"},
		{"MALE", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"MALE","aliases":[],"images":[]}]}}}`, "male"},
		{"TRANSGENDER_MALE", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"TRANSGENDER_MALE","aliases":[],"images":[]}]}}}`, "transgender_male"},
		{"TRANSGENDER_FEMALE", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"TRANSGENDER_FEMALE","aliases":[],"images":[]}]}}}`, "transgender_female"},
		{"INTERSEX", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"INTERSEX","aliases":[],"images":[]}]}}}`, "intersex"},
		{"NON_BINARY", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"NON_BINARY","aliases":[],"images":[]}]}}}`, "non_binary"},
		{"unknown enum", `{"data":{"queryPerformers":{"performers":[{"id":"1","name":"A","gender":"SOMETHING_NEW","aliases":[],"images":[]}]}}}`, "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.fixture
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(body)) //nolint:errcheck
			}))
			defer srv.Close()

			people, err := newPeopleTestAdapter(srv).SearchPeople(context.Background(), "A", 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := people[0].Metadata["gender"]
			if !ok {
				t.Fatal("metadata[gender]: missing")
			}
			if got != tc.wantGender {
				t.Errorf("gender: want %q, got %q", tc.wantGender, got)
			}
		})
	}
}

func TestSearchPeople_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newPeopleTestAdapter(srv).SearchPeople(context.Background(), "any", 10)
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestSearchPeople_BodyModLocationOnly(t *testing.T) {
	fixture := `{"data":{"queryPerformers":{"performers":[{
		"id": "perf-loc",
		"name": "Loc Only",
		"aliases": [],
		"images": [],
		"tattoos": [{"location": "wrist", "description": ""}],
		"piercings": [{"location": "nose", "description": ""}]
	}]}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixture)) //nolint:errcheck
	}))
	defer srv.Close()

	people, err := newPeopleTestAdapter(srv).SearchPeople(context.Background(), "Loc", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("expected 1 person, got %d", len(people))
	}
	if got := people[0].Metadata["tattoos"]; got != "wrist" {
		t.Errorf("tattoos = %q, want wrist", got)
	}
	if got := people[0].Metadata["piercings"]; got != "nose" {
		t.Errorf("piercings = %q, want nose", got)
	}
}

func TestSearchPeople_NilMetadataWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchPeopleNoMetaFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	people, err := newPeopleTestAdapter(srv).SearchPeople(context.Background(), "No Meta", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("expected 1 person, got %d", len(people))
	}
	if people[0].Metadata != nil {
		t.Errorf("expected nil metadata for performer with no fields, got %v", people[0].Metadata)
	}
}
