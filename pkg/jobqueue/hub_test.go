package jobqueue

import (
	"log/slog"
	"testing"
)

func TestHub_SubscribePublishDelivers(t *testing.T) {
	h := newHub(slog.Default())
	ch, _, unsubscribe := h.subscribe("job-1")
	defer unsubscribe()

	evt := &Event{Kind: EventKindTask, TaskID: "task-1", Job: &Job{ID: "job-1", Status: StatusRunning}}
	h.publish(evt)

	select {
	case got := <-ch:
		if got != evt {
			t.Fatalf("publish delivered %v, want %v", got, evt)
		}
	default:
		t.Fatal("subscriber channel had nothing after publish")
	}
}

func TestHub_PublishOnlyReachesSubscribersOfThatJob(t *testing.T) {
	h := newHub(slog.Default())
	chA, _, unsubA := h.subscribe("job-a")
	defer unsubA()
	chB, _, unsubB := h.subscribe("job-b")
	defer unsubB()

	h.publish(&Event{Kind: EventKindJob, Job: &Job{ID: "job-a", Status: StatusRunning}})

	select {
	case <-chA:
	default:
		t.Fatal("job-a's subscriber received nothing")
	}
	select {
	case evt := <-chB:
		t.Fatalf("job-b's subscriber received an event meant for job-a: %v", evt)
	default:
	}
}

func TestHub_MultipleSubscribersOfSameJobBothReceive(t *testing.T) {
	h := newHub(slog.Default())
	ch1, _, unsub1 := h.subscribe("job-1")
	defer unsub1()
	ch2, _, unsub2 := h.subscribe("job-1")
	defer unsub2()

	evt := &Event{Kind: EventKindJob, Job: &Job{ID: "job-1", Status: StatusRunning}}
	h.publish(evt)

	for name, ch := range map[string]chan *Event{"ch1": ch1, "ch2": ch2} {
		select {
		case got := <-ch:
			if got != evt {
				t.Fatalf("%s received %v, want %v", name, got, evt)
			}
		default:
			t.Fatalf("%s received nothing", name)
		}
	}
}

func TestHub_PublishClosesAllSubscribersOnTerminalStatus(t *testing.T) {
	h := newHub(slog.Default())
	ch1, _, _ := h.subscribe("job-1")
	ch2, _, _ := h.subscribe("job-1")

	h.publish(&Event{Kind: EventKindJob, Job: &Job{ID: "job-1", Status: StatusSucceeded}})

	for name, ch := range map[string]chan *Event{"ch1": ch1, "ch2": ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				// the terminal Event itself is the one buffered value;
				// draining it must be followed by a closed channel.
				if _, ok := <-ch; ok {
					t.Fatalf("%s: channel still open after draining the terminal event", name)
				}
			}
		default:
			t.Fatalf("%s: channel had nothing, want at least the terminal event", name)
		}
	}
}

func TestHub_UnsubscribeIsIdempotentAndSafeAfterTerminalClose(_ *testing.T) {
	h := newHub(slog.Default())
	_, _, unsubscribe := h.subscribe("job-1")

	h.publish(&Event{Kind: EventKindJob, Job: &Job{ID: "job-1", Status: StatusSucceeded}})

	// publish already removed+closed the subscriber; calling unsubscribe
	// afterward must not double-close (which would panic) or otherwise
	// misbehave.
	unsubscribe()
	unsubscribe()
}

func TestHub_RemoveOnUnknownJobIsNoop(_ *testing.T) {
	h := newHub(slog.Default())
	h.remove("no-such-job", 0)
}

func TestHub_PublishDropsBeyondSubscriberBufferInsteadOfBlocking(t *testing.T) {
	h := newHub(slog.Default())
	ch, _, unsubscribe := h.subscribe("job-1")
	defer unsubscribe()

	for i := 0; i < subscriberBuffer+5; i++ {
		h.publish(&Event{Kind: EventKindStep, Job: &Job{ID: "job-1", Status: StatusRunning}})
	}

	if len(ch) != subscriberBuffer {
		t.Fatalf("subscriber buffer holds %d events, want exactly the bound %d (extras must be dropped, not block publish)", len(ch), subscriberBuffer)
	}
}
