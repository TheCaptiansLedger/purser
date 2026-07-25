package jobqueue

import (
	"errors"
	"testing"
)

func TestFinalStatus(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		execErr error
		want    Status
	}{
		{name: "executor error", job: &Job{Tasks: []*Task{{Status: StatusSucceeded}}}, execErr: errors.New("boom"), want: StatusFailed},
		{name: "no tasks", job: &Job{}, want: StatusSucceeded},
		{name: "all succeeded", job: &Job{Tasks: []*Task{{Status: StatusSucceeded}, {Status: StatusSucceeded}}}, want: StatusSucceeded},
		{name: "all failed", job: &Job{Tasks: []*Task{{Status: StatusFailed}, {Status: StatusFailed}}}, want: StatusFailed},
		{name: "mixed", job: &Job{Tasks: []*Task{{Status: StatusSucceeded}, {Status: StatusFailed}}}, want: StatusPartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := finalStatus(tt.job, tt.execErr); got != tt.want {
				t.Fatalf("finalStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
