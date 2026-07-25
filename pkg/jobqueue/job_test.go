package jobqueue

import "testing"

func TestTask_Progress(t *testing.T) {
	tests := []struct {
		name string
		task *Task
		want float64
	}{
		{name: "nil task", task: nil, want: 0},
		{name: "no steps", task: &Task{}, want: 0},
		{name: "no steps terminal", task: &Task{Steps: []*Step{{Status: StatusRunning}, {Status: StatusPending}}}, want: 0},
		{
			name: "half terminal",
			task: &Task{Steps: []*Step{{Status: StatusSucceeded}, {Status: StatusRunning}}},
			want: 0.5,
		},
		{
			name: "all terminal",
			task: &Task{Steps: []*Step{{Status: StatusSucceeded}, {Status: StatusFailed}}},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.Progress(); got != tt.want {
				t.Fatalf("Progress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJob_Progress(t *testing.T) {
	tests := []struct {
		name string
		job  *Job
		want float64
	}{
		{name: "nil job", job: nil, want: 0},
		{name: "no tasks", job: &Job{}, want: 0},
		{
			name: "half terminal",
			job:  &Job{Tasks: []*Task{{Status: StatusSucceeded}, {Status: StatusRunning}}},
			want: 0.5,
		},
		{
			name: "all terminal",
			job:  &Job{Tasks: []*Task{{Status: StatusSucceeded}, {Status: StatusFailed}, {Status: StatusPartial}}},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.Progress(); got != tt.want {
				t.Fatalf("Progress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJob_Clone(t *testing.T) {
	j := &Job{
		ID:   "job-1",
		Kind: "diagnostic",
		Tasks: []*Task{
			{ID: "task-1", Label: "one", Steps: []*Step{
				{ID: "step-1", Name: "step", Detail: map[string]string{"k": "v"}},
			}},
		},
	}

	clone := j.Clone()
	clone.Kind = "mutated"
	clone.Tasks[0].Label = "mutated"
	clone.Tasks[0].Steps[0].Detail["k"] = "mutated"

	if j.Kind == "mutated" {
		t.Fatal("mutating the clone's Kind leaked into the original")
	}
	if j.Tasks[0].Label == "mutated" {
		t.Fatal("mutating the clone's Task leaked into the original")
	}
	if j.Tasks[0].Steps[0].Detail["k"] == "mutated" {
		t.Fatal("mutating the clone's Step.Detail leaked into the original")
	}

	if nilClone := (*Job)(nil).Clone(); nilClone != nil {
		t.Fatal("Clone on a nil Job should return nil")
	}
}
