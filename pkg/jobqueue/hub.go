package jobqueue

import (
	"log/slog"
	"sync"
)

// EventKind identifies which level of a Job's Job/Task/Step hierarchy an
// Event describes.
type EventKind string

// The levels of a Job's Job/Task/Step hierarchy an Event can describe.
const (
	EventKindJob  EventKind = "job"
	EventKindTask EventKind = "task"
	EventKindStep EventKind = "step"
)

// Event is one Job/Task/Step transition published to a Job's Watch
// subscribers. Job is always a full, current snapshot — the same shape Get
// returns — so a caller never needs to reassemble partial state out of a
// sequence of Events; TaskID/StepID identify which part of it just
// changed, empty for a job-level transition. See
// docs/adr/0023-job-queue.md.
type Event struct {
	Kind   EventKind
	TaskID string
	StepID string
	Job    *Job
}

// subscriberBuffer bounds how many pending Events a subscriber can
// accumulate before further Events for it are dropped (logged, not
// blocked) rather than stalling the goroutine actually driving the Job.
const subscriberBuffer = 32

type subscriber struct {
	id int
	ch chan *Event
}

// hub tracks Watch subscribers per Job ID and publishes Events to them.
// Every operation is synchronous, invoked from whatever goroutine is
// already driving the Job (Runner, or Engine.run's own job-level
// transitions) or from Engine.Watch itself — hub never starts a goroutine
// of its own, so nothing here can leak one.
type hub struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]*subscriber // jobID -> subscriberID -> subscriber
	logger *slog.Logger
}

func newHub(logger *slog.Logger) *hub {
	return &hub{subs: make(map[string]map[int]*subscriber), logger: logger}
}

// subscribe registers a new subscriber for jobID and returns its Event
// channel plus an unsubscribe func. unsubscribe is safe to call more than
// once — from a deferred cleanup and, independently, from publish once the
// Job reaches a terminal status — and safe even if the subscriber was
// never delivered anything.
func (h *hub) subscribe(jobID string) (chan *Event, func()) {
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	sub := &subscriber{id: id, ch: make(chan *Event, subscriberBuffer)}
	if h.subs[jobID] == nil {
		h.subs[jobID] = make(map[int]*subscriber)
	}
	h.subs[jobID][id] = sub
	h.mu.Unlock()

	return sub.ch, func() { h.remove(jobID, id) }
}

// remove deletes and closes the subscriber id for jobID if still
// registered, a no-op otherwise — what makes it safe to call from both an
// explicit unsubscribe and publish's terminal-Job cleanup without risking
// a double-close panic.
func (h *hub) remove(jobID string, id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.subs[jobID]
	if !ok {
		return
	}
	if s, ok := subs[id]; ok {
		delete(subs, id)
		close(s.ch)
	}
	if len(subs) == 0 {
		delete(h.subs, jobID)
	}
}

// publish delivers evt to every current subscriber of evt.Job.ID. Delivery
// is non-blocking: a subscriber that hasn't drained its buffer misses the
// event (logged at Warn) rather than stalling the Job. If evt.Job.Status is
// terminal, every one of that Job's subscriber channels is closed after
// delivery — a finished Job produces no further Events, so nothing else
// would ever close them, and a WatchJob stream must still end.
func (h *hub) publish(evt *Event) {
	h.mu.Lock()
	jobSubs := h.subs[evt.Job.ID]
	subs := make([]*subscriber, 0, len(jobSubs))
	for _, s := range jobSubs {
		subs = append(subs, s)
	}
	terminal := evt.Job.Status.terminal()
	h.mu.Unlock()

	for _, s := range subs {
		select {
		case s.ch <- evt:
		default:
			h.logger.Warn("jobqueue: dropping event for slow watcher", "job.id", evt.Job.ID, "subscriber_id", s.id)
		}
		if terminal {
			h.remove(evt.Job.ID, s.id)
		}
	}
}
