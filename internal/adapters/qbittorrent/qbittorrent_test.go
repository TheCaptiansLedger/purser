package qbittorrent_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"purser/internal/adapters/qbittorrent"
	"purser/internal/ports"
	"purser/internal/ports/downloadclienttest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

// Real qBittorrent WebUI API paths this adapter's request-building code
// hits — see qbittorrent.go's package doc comment.
const (
	loginPath  = "/api/v2/auth/login"
	addPath    = "/api/v2/torrents/add"
	infoPath   = "/api/v2/torrents/info"
	deletePath = "/api/v2/torrents/delete"
)

func newTestClient(t *testing.T, baseURL string) *qbittorrent.Client {
	t.Helper()
	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Username = "qbit-user"
	cfg.Password = "qbit-pass"
	c, err := qbittorrent.New(cfg, qbittorrent.WithTagLookupInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("qbittorrent.New returned error: %v", err)
	}
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...qbittorrent.Option) *qbittorrent.Client {
	t.Helper()
	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = "http://qbittorrent.invalid"
	cfg.Username = "qbit-user"
	cfg.Password = "qbit-pass"
	allOpts := append([]qbittorrent.Option{qbittorrent.WithBaseTransport(rt), qbittorrent.WithTagLookupInterval(time.Millisecond)}, opts...)
	c, err := qbittorrent.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("qbittorrent.New returned error: %v", err)
	}
	return c
}

// loginResponder answers POST /api/v2/auth/login the way a real qBittorrent
// instance does on success: 200, plain-text "Ok." body, and a Set-Cookie
// SID header — the shape login() actually parses.
func loginResponder(sid string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Content-Type": []string{"text/plain"},
				"Set-Cookie":   []string{"SID=" + sid + "; Path=/"},
			},
			Body:    io.NopCloser(strings.NewReader("Ok.")),
			Request: req,
		}, nil
	}
}

func plainText(status int, body string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	}
}

func TestClient_DownloadClientContract(t *testing.T) {
	downloadclienttest.TestDownloadClient(t, func(t *testing.T, baseURL string) ports.DownloadClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := qbittorrent.DefaultConfig()
	cfg.Username = "u"
	cfg.Password = "p"
	if _, err := qbittorrent.New(cfg); err == nil {
		t.Fatal("qbittorrent.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyUsername(t *testing.T) {
	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = "http://qbittorrent.invalid"
	cfg.Password = "p"
	if _, err := qbittorrent.New(cfg); err == nil {
		t.Fatal("qbittorrent.New with empty Username returned nil error")
	}
}

func TestNew_RejectsEmptyPassword(t *testing.T) {
	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = "http://qbittorrent.invalid"
	cfg.Username = "u"
	if _, err := qbittorrent.New(cfg); err == nil {
		t.Fatal("qbittorrent.New with empty Password returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodPost,
			Path:   loginPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				gotUA = req.Header.Get("User-Agent")
				return loginResponder("session-1")(req)
			},
		},
		httpmock.Route{Method: http.MethodGet, Path: infoPath, Responder: httpmock.JSON(http.StatusOK, []map[string]any{})},
	)

	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = "http://qbittorrent.invalid"
	cfg.Username = "qbit-user"
	cfg.Password = "qbit-pass"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := qbittorrent.New(cfg, qbittorrent.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("qbittorrent.New returned error: %v", err)
	}

	_, err = c.Status(context.Background(), "some-hash")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound for an empty info result", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestClient_LoginSendsFormEncodedCredentialsNotQueryParamOrBearer(t *testing.T) {
	var gotUsername, gotPassword, gotContentType, gotAuthHeader, gotQueryUser string
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodPost,
			Path:   loginPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				gotContentType = req.Header.Get("Content-Type")
				gotAuthHeader = req.Header.Get("Authorization")
				gotQueryUser = req.URL.Query().Get("username")
				_ = req.ParseForm()
				gotUsername = req.PostFormValue("username")
				gotPassword = req.PostFormValue("password")
				return loginResponder("session-1")(req)
			},
		},
		httpmock.Route{Method: http.MethodGet, Path: infoPath, Responder: httpmock.JSON(http.StatusOK, []map[string]any{})},
	)

	c := newMockedTestClient(t, rt)
	_, err := c.Status(context.Background(), "some-hash")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound", err)
	}

	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("login Content-Type = %q, want application/x-www-form-urlencoded", gotContentType)
	}
	if gotUsername != "qbit-user" || gotPassword != "qbit-pass" {
		t.Errorf("login username/password = %q/%q, want qbit-user/qbit-pass", gotUsername, gotPassword)
	}
	if gotAuthHeader != "" {
		t.Errorf("Authorization header = %q, want empty (qBittorrent uses a session cookie, not bearer auth)", gotAuthHeader)
	}
	if gotQueryUser != "" {
		t.Errorf("username query param = %q, want empty (credentials must not leak into the URL)", gotQueryUser)
	}
}

func TestClient_LoginFailedCredentialsReturnsError(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: plainText(http.StatusOK, "Fails.")},
	)

	c := newMockedTestClient(t, rt)
	_, err := c.Status(context.Background(), "some-hash")
	if err == nil {
		t.Fatal("Status with failed login returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("Status error = %v, want a plain login error, not ports.ErrNotFound", err)
	}
}

func TestClient_LoginMissingSessionCookieReturnsError(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: plainText(http.StatusOK, "Ok.")},
	)

	c := newMockedTestClient(t, rt)
	_, err := c.Status(context.Background(), "some-hash")
	if err == nil {
		t.Fatal("Status with no Set-Cookie on login returned nil error")
	}
}

func TestClient_ReauthenticatesOnceOnExpiredSession(t *testing.T) {
	var loginCalls, infoCalls int
	rt := httpmock.New(
		httpmock.Route{
			Method: http.MethodPost,
			Path:   loginPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				loginCalls++
				return loginResponder("session-1")(req)
			},
		},
		httpmock.Route{
			Method: http.MethodGet,
			Path:   infoPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				infoCalls++
				if infoCalls == 1 {
					return httpmock.Status(http.StatusForbidden)(req)
				}
				return httpmock.JSON(http.StatusOK, []map[string]any{
					{"hash": "known-hash", "state": "downloading", "progress": 0.5, "save_path": "/downloads/x", "eta": 100},
				})(req)
			},
		},
	)

	c := newMockedTestClient(t, rt)
	status, err := c.Status(context.Background(), "known-hash")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ExternalID != "known-hash" {
		t.Errorf("Status.ExternalID = %q, want known-hash", status.ExternalID)
	}
	if loginCalls != 2 {
		t.Errorf("login calls = %d, want 2 (initial + re-auth after 403)", loginCalls)
	}
	if infoCalls != 2 {
		t.Errorf("info calls = %d, want 2 (initial 403 + retry)", infoCalls)
	}
}

func TestAdd_SendsMultipartFormAndReturnsHashFoundByTagLookup(t *testing.T) {
	var gotURLs, gotCategory, gotTags string
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{
			Method: http.MethodPost,
			Path:   addPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				_ = req.ParseMultipartForm(1 << 20)
				gotURLs = req.FormValue("urls")
				gotCategory = req.FormValue("category")
				gotTags = req.FormValue("tags")
				return plainText(http.StatusOK, "Ok.")(req)
			},
		},
		httpmock.Route{
			Method: http.MethodGet,
			Path:   infoPath,
			Responder: httpmock.JSON(http.StatusOK, []map[string]any{
				{"hash": "found-hash-xyz", "state": "metaDL", "progress": 0, "save_path": "/downloads/y", "eta": 8640000},
			}),
		},
	)

	c := newMockedTestClient(t, rt)
	id, err := c.Add(context.Background(), ports.AddDownloadRequest{
		DownloadURL: "magnet:?xt=urn:btih:abc123",
		Title:       "Some Release",
		Category:    "music",
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if id != "found-hash-xyz" {
		t.Errorf("Add id = %q, want found-hash-xyz", id)
	}
	if gotURLs != "magnet:?xt=urn:btih:abc123" {
		t.Errorf("urls field = %q, want the magnet DownloadURL", gotURLs)
	}
	if gotCategory != "music" {
		t.Errorf("category field = %q, want music (passed through as-is)", gotCategory)
	}
	if gotTags == "" {
		t.Error("tags field is empty, want a generated lookup tag")
	}
}

func TestAdd_EmptyCategoryOmitsField(t *testing.T) {
	var sawCategory bool
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{
			Method: http.MethodPost,
			Path:   addPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				_ = req.ParseMultipartForm(1 << 20)
				_, sawCategory = req.MultipartForm.Value["category"]
				return plainText(http.StatusOK, "Ok.")(req)
			},
		},
		httpmock.Route{
			Method:    http.MethodGet,
			Path:      infoPath,
			Responder: httpmock.JSON(http.StatusOK, []map[string]any{{"hash": "h", "state": "downloading"}}),
		},
	)

	c := newMockedTestClient(t, rt)
	if _, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "magnet:?xt=urn:btih:abc", Title: "x"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if sawCategory {
		t.Error("category field present with an empty AddDownloadRequest.Category, want omitted")
	}
}

func TestAdd_QBittorrentReportedFailureReturnsError(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{Method: http.MethodPost, Path: addPath, Responder: plainText(http.StatusOK, "Fails.")},
	)

	c := newMockedTestClient(t, rt)
	_, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "magnet:?xt=urn:btih:abc", Title: "x"})
	if err == nil {
		t.Fatal("Add with a qBittorrent-reported failure returned nil error")
	}
}

func TestAdd_NoTorrentFoundByTagEventuallyErrors(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{Method: http.MethodPost, Path: addPath, Responder: plainText(http.StatusOK, "Ok.")},
		httpmock.Route{Method: http.MethodGet, Path: infoPath, Responder: httpmock.JSON(http.StatusOK, []map[string]any{})},
	)

	c := newMockedTestClient(t, rt, qbittorrent.WithTagLookupMaxAttempts(2), qbittorrent.WithTagLookupInterval(time.Millisecond))
	_, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "magnet:?xt=urn:btih:abc", Title: "x"})
	if err == nil {
		t.Fatal("Add with no torrent ever found by tag returned nil error")
	}
}

func TestStatus_MapsRealQBittorrentStateVocabulary(t *testing.T) {
	tests := []struct {
		qbState string
		want    ports.DownloadState
	}{
		{"downloading", ports.DownloadStateDownloading},
		{"metaDL", ports.DownloadStateDownloading},
		{"forcedMetaDL", ports.DownloadStateDownloading},
		{"forcedDL", ports.DownloadStateDownloading},
		{"allocating", ports.DownloadStateDownloading},
		{"moving", ports.DownloadStateDownloading},
		{"checkingDL", ports.DownloadStateDownloading},
		{"pausedDL", ports.DownloadStatePaused},
		{"queuedDL", ports.DownloadStateQueued},
		{"stalledDL", ports.DownloadStateQueued},
		{"uploading", ports.DownloadStateCompleted},
		{"stalledUP", ports.DownloadStateCompleted},
		{"checkingUP", ports.DownloadStateCompleted},
		{"forcedUP", ports.DownloadStateCompleted},
		{"pausedUP", ports.DownloadStateCompleted},
		{"queuedUP", ports.DownloadStateCompleted},
		{"error", ports.DownloadStateFailed},
		{"missingFiles", ports.DownloadStateFailed},
		{"checkingResumeData", ports.DownloadStateQueued},
		{"unknown", ports.DownloadStateQueued},
	}

	for _, tt := range tests {
		t.Run(tt.qbState, func(t *testing.T) {
			rt := httpmock.New(
				httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
				httpmock.Route{
					Method: http.MethodGet,
					Path:   infoPath,
					Responder: httpmock.JSON(http.StatusOK, []map[string]any{
						{"hash": "h", "state": tt.qbState, "progress": 0.1, "save_path": "/x", "eta": 100},
					}),
				},
			)
			c := newMockedTestClient(t, rt)
			status, err := c.Status(context.Background(), "h")
			if err != nil {
				t.Fatalf("Status returned error: %v", err)
			}
			if status.State != tt.want {
				t.Errorf("state %q mapped to %q, want %q", tt.qbState, status.State, tt.want)
			}
		})
	}
}

func TestStatus_ETAInfinitySentinelDegradesToNilETA(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{
			Method: http.MethodGet,
			Path:   infoPath,
			Responder: httpmock.JSON(http.StatusOK, []map[string]any{
				{"hash": "h", "state": "stalledDL", "progress": 0, "save_path": "/x", "eta": 8640000},
			}),
		},
	)
	c := newMockedTestClient(t, rt)
	status, err := c.Status(context.Background(), "h")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ETA != nil {
		t.Errorf("ETA = %v, want nil for qBittorrent's 8640000 sentinel", status.ETA)
	}
}

func TestStatus_RealETADegradesToNonNilDuration(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{
			Method: http.MethodGet,
			Path:   infoPath,
			Responder: httpmock.JSON(http.StatusOK, []map[string]any{
				{"hash": "h", "state": "downloading", "progress": 0.2, "save_path": "/x", "eta": 120},
			}),
		},
	)
	c := newMockedTestClient(t, rt)
	status, err := c.Status(context.Background(), "h")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ETA == nil || *status.ETA != 120*time.Second {
		t.Errorf("ETA = %v, want 120s", status.ETA)
	}
}

func TestRemove_SendsHashesAndDeleteFilesForm(t *testing.T) {
	var gotHashes, gotDeleteFiles string
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{
			Method: http.MethodGet,
			Path:   infoPath,
			Responder: httpmock.JSON(http.StatusOK, []map[string]any{
				{"hash": "known-hash", "state": "downloading"},
			}),
		},
		httpmock.Route{
			Method: http.MethodPost,
			Path:   deletePath,
			Responder: func(req *http.Request) (*http.Response, error) {
				_ = req.ParseForm()
				gotHashes = req.PostFormValue("hashes")
				gotDeleteFiles = req.PostFormValue("deleteFiles")
				return plainText(http.StatusOK, "Ok.")(req)
			},
		},
	)

	c := newMockedTestClient(t, rt)
	if err := c.Remove(context.Background(), "known-hash", true); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if gotHashes != "known-hash" {
		t.Errorf("hashes field = %q, want known-hash", gotHashes)
	}
	if gotDeleteFiles != "true" {
		t.Errorf("deleteFiles field = %q, want true", gotDeleteFiles)
	}
}

// TestRemove_UnknownHashReturnsErrNotFoundWithoutCallingDelete relies on
// deletePath having no registered route at all: if Remove incorrectly
// called delete for an unknown hash, httpmock.Transport would fail loudly
// ("no route registered") instead of the test silently passing.
func TestRemove_UnknownHashReturnsErrNotFoundWithoutCallingDelete(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{Method: http.MethodGet, Path: infoPath, Responder: httpmock.JSON(http.StatusOK, []map[string]any{})},
	)

	c := newMockedTestClient(t, rt)
	err := c.Remove(context.Background(), "unknown-hash", false)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Remove error = %v, want ports.ErrNotFound", err)
	}
}

func TestProtocol_ReturnsTorrent(t *testing.T) {
	c := newMockedTestClient(t, httpmock.New())
	if p := c.Protocol(); p != ports.ProtocolTorrent {
		t.Errorf("Protocol() = %q, want %q", p, ports.ProtocolTorrent)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodPost, Path: loginPath, Responder: loginResponder("session-1")},
		httpmock.Route{Method: http.MethodGet, Path: infoPath, Responder: httpmock.JSON(http.StatusOK, []map[string]any{})},
	)

	c := newMockedTestClient(t, rt,
		qbittorrent.WithLogger(slog.Default()),
		qbittorrent.WithTracerProvider(otel.GetTracerProvider()),
		qbittorrent.WithMeterProvider(otel.GetMeterProvider()),
	)

	_, err := c.Status(context.Background(), "h")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound", err)
	}
}
