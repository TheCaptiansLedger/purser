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
		Long: "Opens a full-screen, split-pane terminal UI against a running purser\n" +
			"serve instance's purser.job.v1.JobService: a polled, scrollable list of\n" +
			"recent jobs on the left, and a live-streamed (WatchJob) detail view of\n" +
			"the highlighted job's tasks and steps on the right. Tab switches focus\n" +
			"between panes. See docs/adr/0023-job-queue.md.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := jobv1connect.NewJobServiceClient(httpClient, addr)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			m := newJobsModel(ctx, client)
			m.addr = addr
			_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
			return err
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	return cmd
}
