package memory_test

import (
	"purser/pkg/jobqueue"
	"purser/pkg/jobqueue/jobqueuetest"
	"purser/pkg/jobqueue/memory"
	"testing"
)

func TestStore_Contract(t *testing.T) {
	jobqueuetest.TestStore(t, func(t *testing.T) jobqueue.Store {
		t.Helper()
		return memory.New()
	})
}
