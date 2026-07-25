package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeScanService struct {
	gotRoot  string
	returnID string
	err      error
}

func (f *fakeScanService) Trigger(_ context.Context, root string) (string, error) {
	f.gotRoot = root
	if f.err != nil {
		return "", f.err
	}
	return f.returnID, nil
}

func TestScanHandler_TriggerScan(t *testing.T) {
	t.Run("valid request returns the job id", func(t *testing.T) {
		svc := &fakeScanService{returnID: "job-1"}
		h := apiconnect.NewScanHandler(svc, nil)

		res, err := h.TriggerScan(context.Background(), connect.NewRequest(&pipelinev1.TriggerScanRequest{Root: "/media/new-arrivals"}))
		if err != nil {
			t.Fatalf("TriggerScan returned error: %v", err)
		}
		if res.Msg.GetJobId() != "job-1" {
			t.Fatalf("TriggerScan returned job_id %q, want %q", res.Msg.GetJobId(), "job-1")
		}
		if svc.gotRoot != "/media/new-arrivals" {
			t.Fatalf("TriggerScan passed root %q, want %q", svc.gotRoot, "/media/new-arrivals")
		}
	})

	t.Run("a not-found-shaped error maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeScanService{err: ports.ErrNotFound}
		h := apiconnect.NewScanHandler(svc, nil)

		_, err := h.TriggerScan(context.Background(), connect.NewRequest(&pipelinev1.TriggerScanRequest{Root: "/missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("TriggerScan with ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeScanService{err: errors.New("boom")}
		h := apiconnect.NewScanHandler(svc, nil)

		_, err := h.TriggerScan(context.Background(), connect.NewRequest(&pipelinev1.TriggerScanRequest{Root: "/media"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("TriggerScan with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
