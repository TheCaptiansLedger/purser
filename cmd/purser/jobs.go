package main

import (
	"context"
	"net/http"
	"purser/gen/go/purser/job/v1/jobv1connect"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func newJobsCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Watch the job queue in an interactive TUI",
		Long: "Opens a full-screen terminal UI against a running purser serve\n" +
			"instance's purser.job.v1.JobService: a polled list of recent jobs, and\n" +
			"a live-streamed (WatchJob) detail view of one job's tasks and steps.\n" +
			"See docs/adr/0023-job-queue.md.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := jobv1connect.NewJobServiceClient(httpClient, addr)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			m := newJobsModel(ctx, client)
			_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
			return err
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	return cmd
}
