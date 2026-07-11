package httpclient

import (
	"crypto/tls"
	"net/http/httptrace"
	"testing"
	"time"
)

func TestPhaseDuration(t *testing.T) {
	base := time.Now()

	tests := []struct {
		name  string
		start time.Time
		done  time.Time
		want  time.Duration
	}{
		{"zero start", time.Time{}, base, 0},
		{"zero done", base, time.Time{}, 0},
		{"both zero", time.Time{}, time.Time{}, 0},
		{"valid range", base, base.Add(5 * time.Millisecond), 5 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := phaseDuration(tt.start, tt.done); got != tt.want {
				t.Errorf("phaseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTraceHooks_DurationsReflectRecordedTimestamps(t *testing.T) {
	h := newTraceHooks()
	base := time.Now()

	h.dnsStart = base
	h.dnsDone = base.Add(1 * time.Millisecond)
	h.connectStart = base.Add(2 * time.Millisecond)
	h.connectDone = base.Add(5 * time.Millisecond)
	h.tlsStart = base.Add(5 * time.Millisecond)
	h.tlsDone = base.Add(9 * time.Millisecond)

	if got := h.dnsDuration(); got != time.Millisecond {
		t.Errorf("dnsDuration() = %v, want 1ms", got)
	}
	if got := h.connectDuration(); got != 3*time.Millisecond {
		t.Errorf("connectDuration() = %v, want 3ms", got)
	}
	if got := h.tlsDuration(); got != 4*time.Millisecond {
		t.Errorf("tlsDuration() = %v, want 4ms", got)
	}
}

func TestTraceHooks_ClientTraceCallbacksUpdateState(t *testing.T) {
	h := newTraceHooks()
	ct := h.clientTrace()

	ct.DNSStart(httptrace.DNSStartInfo{Host: "example.invalid"})
	time.Sleep(time.Millisecond)
	ct.DNSDone(httptrace.DNSDoneInfo{})

	ct.ConnectStart("tcp", "127.0.0.1:0")
	time.Sleep(time.Millisecond)
	ct.ConnectDone("tcp", "127.0.0.1:0", nil)

	ct.TLSHandshakeStart()
	time.Sleep(time.Millisecond)
	ct.TLSHandshakeDone(tls.ConnectionState{}, nil)

	if h.dnsDuration() <= 0 {
		t.Error("dnsDuration() should be > 0 after DNSStart/DNSDone")
	}
	if h.connectDuration() <= 0 {
		t.Error("connectDuration() should be > 0 after ConnectStart/ConnectDone")
	}
	if h.tlsDuration() <= 0 {
		t.Error("tlsDuration() should be > 0 after TLSHandshakeStart/TLSHandshakeDone")
	}
}
