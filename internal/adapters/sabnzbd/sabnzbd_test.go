package sabnzbd_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"purser/internal/adapters/sabnzbd"
	"purser/internal/ports"
	"purser/internal/ports/downloadclienttest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

// apiPath is the single real SABnzbd API endpoint this adapter's
// request-building code hits — see sabnzbd.go's package doc comment.
const apiPath = "/api"

func newTestClient(t *testing.T, baseURL string) *sabnzbd.Client {
	t.Helper()
	cfg := sabnzbd.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := sabnzbd.New(cfg)
	if err != nil {
		t.Fatalf("sabnzbd.New returned error: %v", err)
	}
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...sabnzbd.Option) *sabnzbd.Client {
	t.Helper()
	cfg := sabnzbd.DefaultConfig()
	cfg.BaseURL = "http://sabnzbd.invalid"
	cfg.APIKey = "test-key"
	allOpts := append([]sabnzbd.Option{sabnzbd.WithBaseTransport(rt)}, opts...)
	c, err := sabnzbd.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("sabnzbd.New returned error: %v", err)
	}
	return c
}

func TestClient_DownloadClientContract(t *testing.T) {
	downloadclienttest.TestDownloadClient(t, ports.ProtocolUsenet, func(t *testing.T, baseURL string) ports.DownloadClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := sabnzbd.DefaultConfig()
	cfg.APIKey = "key"
	if _, err := sabnzbd.New(cfg); err == nil {
		t.Fatal("sabnzbd.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := sabnzbd.DefaultConfig()
	cfg.BaseURL = "http://sabnzbd.invalid"
	if _, err := sabnzbd.New(cfg); err == nil {
		t.Fatal("sabnzbd.New with empty APIKey returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				gotUA = req.Header.Get("User-Agent")
				return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{}}})(req)
			},
		},
	)

	cfg := sabnzbd.DefaultConfig()
	cfg.BaseURL = "http://sabnzbd.invalid"
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := sabnzbd.New(cfg, sabnzbd.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("sabnzbd.New returned error: %v", err)
	}

	_, err = c.Status(context.Background(), "unknown-id")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound for an empty queue/history result", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestAdd_SendsAddURLQueryAndReturnsFirstNzoID(t *testing.T) {
	var gotMode, gotName, gotCat, gotAPIKey string
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				gotMode = q.Get("mode")
				gotName = q.Get("name")
				gotCat = q.Get("cat")
				gotAPIKey = q.Get("apikey")
				return httpmock.JSON(http.StatusOK, map[string]any{"status": true, "nzo_ids": []string{"SABnzbd_nzo_abc123"}})(req)
			},
		},
	)

	c := newMockedTestClient(t, rt)
	id, err := c.Add(context.Background(), ports.AddDownloadRequest{
		DownloadURL: "https://example.invalid/release.nzb",
		Title:       "Some Release",
		Category:    "music",
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if id != "SABnzbd_nzo_abc123" {
		t.Errorf("Add id = %q, want SABnzbd_nzo_abc123", id)
	}
	if gotMode != "addurl" {
		t.Errorf("mode = %q, want addurl", gotMode)
	}
	if gotName != "https://example.invalid/release.nzb" {
		t.Errorf("name = %q, want the nzb DownloadURL", gotName)
	}
	if gotCat != "music" {
		t.Errorf("cat = %q, want music (passed through as-is)", gotCat)
	}
	if gotAPIKey != "test-key" {
		t.Errorf("apikey = %q, want test-key", gotAPIKey)
	}
}

func TestAdd_EmptyCategoryOmitsField(t *testing.T) {
	var sawCat bool
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				_, sawCat = req.URL.Query()["cat"]
				return httpmock.JSON(http.StatusOK, map[string]any{"status": true, "nzo_ids": []string{"id"}})(req)
			},
		},
	)

	c := newMockedTestClient(t, rt)
	if _, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "https://example.invalid/x.nzb", Title: "x"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if sawCat {
		t.Error("cat field present with an empty AddDownloadRequest.Category, want omitted")
	}
}

func TestAdd_SABnzbdReportedFailureReturnsError(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodGet, Path: apiPath, Responder: httpmock.JSON(http.StatusOK, map[string]any{"status": false, "error": "URL invalid"})},
	)

	c := newMockedTestClient(t, rt)
	_, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "https://example.invalid/x.nzb", Title: "x"})
	if err == nil {
		t.Fatal("Add with a SABnzbd-reported failure returned nil error")
	}
}

func TestAdd_NoNzoIDsInResponseReturnsError(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodGet, Path: apiPath, Responder: httpmock.JSON(http.StatusOK, map[string]any{"status": true, "nzo_ids": []string{}})},
	)

	c := newMockedTestClient(t, rt)
	_, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "https://example.invalid/x.nzb", Title: "x"})
	if err == nil {
		t.Fatal("Add with no nzo_ids in response returned nil error")
	}
}

func TestStatus_MapsRealQueueStateVocabulary(t *testing.T) {
	tests := []struct {
		sabStatus string
		want      ports.DownloadState
	}{
		{"Downloading", ports.DownloadStateDownloading},
		{"Fetching", ports.DownloadStateDownloading},
		{"Queued", ports.DownloadStateQueued},
		{"Propagating", ports.DownloadStateQueued},
		{"Paused", ports.DownloadStatePaused},
		{"SomeUnknownState", ports.DownloadStateQueued},
	}

	for _, tt := range tests {
		t.Run(tt.sabStatus, func(t *testing.T) {
			rt := httpmock.New(
				httpmock.Route{
					Method: http.MethodGet,
					Path:   apiPath,
					Responder: func(req *http.Request) (*http.Response, error) {
						q := req.URL.Query()
						if q.Get("mode") == "queue" {
							return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{
								map[string]any{"nzo_id": "h", "status": tt.sabStatus, "percentage": "10", "timeleft": "0:05:00"},
							}}})(req)
						}
						return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
					},
				},
			)
			c := newMockedTestClient(t, rt)
			status, err := c.Status(context.Background(), "h")
			if err != nil {
				t.Fatalf("Status returned error: %v", err)
			}
			if status.State != tt.want {
				t.Errorf("state %q mapped to %q, want %q", tt.sabStatus, status.State, tt.want)
			}
		})
	}
}

func TestStatus_MapsRealHistoryStateVocabulary(t *testing.T) {
	tests := []struct {
		sabStatus string
		want      ports.DownloadState
	}{
		{"Completed", ports.DownloadStateCompleted},
		{"Failed", ports.DownloadStateFailed},
		{"QuickCheck", ports.DownloadStateDownloading},
		{"Verifying", ports.DownloadStateDownloading},
		{"Repairing", ports.DownloadStateDownloading},
		{"Extracting", ports.DownloadStateDownloading},
		{"Moving", ports.DownloadStateDownloading},
		{"Running", ports.DownloadStateDownloading},
		{"Fetching", ports.DownloadStateDownloading},
		{"Queued", ports.DownloadStateQueued},
		{"SomeUnknownState", ports.DownloadStateQueued},
	}

	for _, tt := range tests {
		t.Run(tt.sabStatus, func(t *testing.T) {
			rt := httpmock.New(
				httpmock.Route{
					Method: http.MethodGet,
					Path:   apiPath,
					Responder: func(req *http.Request) (*http.Response, error) {
						q := req.URL.Query()
						if q.Get("mode") == "queue" {
							return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{}}})(req)
						}
						return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{
							map[string]any{"nzo_id": "h", "status": tt.sabStatus, "storage": "/downloads/some-release"},
						}}})(req)
					},
				},
			)
			c := newMockedTestClient(t, rt)
			status, err := c.Status(context.Background(), "h")
			if err != nil {
				t.Fatalf("Status returned error: %v", err)
			}
			if status.State != tt.want {
				t.Errorf("state %q mapped to %q, want %q", tt.sabStatus, status.State, tt.want)
			}
			if status.Progress != 1.0 {
				t.Errorf("Progress = %v, want 1.0 for a history slot", status.Progress)
			}
			if status.SavePath != "/downloads/some-release" {
				t.Errorf("SavePath = %q, want /downloads/some-release", status.SavePath)
			}
		})
	}
}

func TestStatus_ParsesTimeLeftIntoETA(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{
						map[string]any{"nzo_id": "h", "status": "Downloading", "percentage": "20", "timeleft": "1:02:03"},
					}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	status, err := c.Status(context.Background(), "h")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	want := time.Hour + 2*time.Minute + 3*time.Second
	if status.ETA == nil || *status.ETA != want {
		t.Errorf("ETA = %v, want %v", status.ETA, want)
	}
	if status.Progress != 0.2 {
		t.Errorf("Progress = %v, want 0.2", status.Progress)
	}
}

func TestStatus_MalformedTimeLeftDegradesToNilETA(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{
						map[string]any{"nzo_id": "h", "status": "Downloading", "percentage": "20", "timeleft": "unknown"},
					}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	status, err := c.Status(context.Background(), "h")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ETA != nil {
		t.Errorf("ETA = %v, want nil for a malformed timeleft", status.ETA)
	}
}

func TestStatus_UnknownIDReturnsErrNotFound(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	_, err := c.Status(context.Background(), "unknown")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound", err)
	}
}

func TestRemove_DeletesFromQueueWhenFoundThere(t *testing.T) {
	var deleteMode, deleteValue, deleteFilesParam string
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("name") == "delete" {
					deleteMode = q.Get("mode")
					deleteValue = q.Get("value")
					deleteFilesParam = q.Get("del_files")
					return httpmock.JSON(http.StatusOK, map[string]any{"status": true})(req)
				}
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{
						map[string]any{"nzo_id": "known-id", "status": "Downloading"},
					}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	if err := c.Remove(context.Background(), "known-id", true); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if deleteMode != "queue" {
		t.Errorf("delete mode = %q, want queue", deleteMode)
	}
	if deleteValue != "known-id" {
		t.Errorf("delete value = %q, want known-id", deleteValue)
	}
	if deleteFilesParam != "1" {
		t.Errorf("del_files = %q, want 1", deleteFilesParam)
	}
}

func TestRemove_DeletesFromHistoryWhenNotInQueue(t *testing.T) {
	var deleteMode string
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("name") == "delete" {
					deleteMode = q.Get("mode")
					return httpmock.JSON(http.StatusOK, map[string]any{"status": true})(req)
				}
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{
					map[string]any{"nzo_id": "known-id", "status": "Completed", "storage": "/downloads/x"},
				}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	if err := c.Remove(context.Background(), "known-id", false); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if deleteMode != "history" {
		t.Errorf("delete mode = %q, want history", deleteMode)
	}
}

// TestRemove_UnknownIDReturnsErrNotFoundWithoutCallingDelete relies on no
// request ever carrying name=delete: if Remove incorrectly called delete
// for an unknown ID, the responder below would fail the test via t.Error.
func TestRemove_UnknownIDReturnsErrNotFoundWithoutCallingDelete(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("name") == "delete" {
					t.Error("delete called for an unknown external ID")
				}
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	err := c.Remove(context.Background(), "unknown-id", false)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Remove error = %v, want ports.ErrNotFound", err)
	}
}

func TestRemove_SABnzbdReportedFailureReturnsError(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodGet,
			Path:   apiPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("name") == "delete" {
					return httpmock.JSON(http.StatusOK, map[string]any{"status": false, "error": "boom"})(req)
				}
				if q.Get("mode") == "queue" {
					return httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{
						map[string]any{"nzo_id": "known-id", "status": "Downloading"},
					}}})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)
			},
		},
	)
	c := newMockedTestClient(t, rt)
	if err := c.Remove(context.Background(), "known-id", false); err == nil {
		t.Fatal("Remove with a SABnzbd-reported delete failure returned nil error")
	}
}

func TestProtocol_ReturnsUsenet(t *testing.T) {
	c := newMockedTestClient(t, httpmock.New())
	if p := c.Protocol(); p != ports.ProtocolUsenet {
		t.Errorf("Protocol() = %q, want %q", p, ports.ProtocolUsenet)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodGet, Path: apiPath, Responder: httpmock.JSON(http.StatusOK, map[string]any{"queue": map[string]any{"slots": []any{}}})},
	)

	c := newMockedTestClient(t, rt,
		sabnzbd.WithLogger(slog.Default()),
		sabnzbd.WithTracerProvider(otel.GetTracerProvider()),
		sabnzbd.WithMeterProvider(otel.GetMeterProvider()),
	)

	_, err := c.Status(context.Background(), "h")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound", err)
	}
}

func TestApp_HTTPStatusErrorPropagates(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodGet, Path: apiPath, Responder: httpmock.Status(http.StatusInternalServerError)},
	)
	c := newMockedTestClient(t, rt)
	_, err := c.Status(context.Background(), "h")
	if err == nil {
		t.Fatal("Status against a 500 response returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Error("Status error wraps ports.ErrNotFound for an HTTP 500, want a plain error")
	}
}
