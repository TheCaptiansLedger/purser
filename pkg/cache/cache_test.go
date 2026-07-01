package cache_test

import (
	"purser/pkg/cache"
	"testing"
	"time"
)

func newCache(t *testing.T) *cache.Cache {
	t.Helper()
	c, err := cache.New("test", 128)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCache_MissOnEmpty(t *testing.T) {
	_, ok := newCache(t).Get("x")
	if ok {
		t.Fatal("expected miss on empty cache")
	}
}

func TestCache_SetThenGet(t *testing.T) {
	c := newCache(t)
	c.Set("k", []byte("v"), time.Minute)
	got, ok := c.Get("k")
	if !ok {
		t.Fatal("expected hit after Set")
	}
	if string(got) != "v" {
		t.Errorf("value = %q, want %q", got, "v")
	}
}

func TestCache_ExpiredEntryMisses(t *testing.T) {
	c := newCache(t)
	c.Set("k", []byte("v"), -time.Second)
	_, ok := c.Get("k")
	if ok {
		t.Fatal("expected miss for expired entry")
	}
}

func TestCache_LRUEviction(t *testing.T) {
	c, err := cache.New("test", 2)
	if err != nil {
		t.Fatal(err)
	}
	c.Set("a", []byte("1"), time.Minute)
	c.Set("b", []byte("2"), time.Minute)
	c.Set("c", []byte("3"), time.Minute) // evicts LRU entry ("a")
	_, ok := c.Get("a")
	if ok {
		t.Fatal("expected 'a' to be evicted by LRU policy")
	}
	_, ok = c.Get("c")
	if !ok {
		t.Fatal("expected 'c' to be present after LRU eviction of 'a'")
	}
}

func TestCache_Stats(t *testing.T) {
	c := newCache(t)
	c.Set("a", []byte("1"), time.Minute)
	c.Get("a") // hit
	c.Get("b") // miss

	s := c.Stats()
	if s.Hits != 1 {
		t.Errorf("Hits = %d, want 1", s.Hits)
	}
	if s.Misses != 1 {
		t.Errorf("Misses = %d, want 1", s.Misses)
	}
	if s.Size != 1 {
		t.Errorf("Size = %d, want 1", s.Size)
	}
}

func TestCache_StatsIncludesName(t *testing.T) {
	c, err := cache.New("my-cache", 128)
	if err != nil {
		t.Fatal(err)
	}
	if s := c.Stats(); s.Name != "my-cache" {
		t.Errorf("Stats.Name = %q, want %q", s.Name, "my-cache")
	}
}

func TestCache_StatsExpiredCountsAsMiss(t *testing.T) {
	c := newCache(t)
	c.Set("k", []byte("v"), -time.Second)
	c.Get("k")

	s := c.Stats()
	if s.Misses != 1 {
		t.Errorf("Misses = %d, want 1 for expired entry", s.Misses)
	}
	if s.Hits != 0 {
		t.Errorf("Hits = %d, want 0 for expired entry", s.Hits)
	}
}

func TestCache_BytesTrackedOnSet(t *testing.T) {
	c := newCache(t)
	c.Set("k", []byte("hello"), time.Minute) // 5 bytes
	if b := c.Stats().Bytes; b != 5 {
		t.Errorf("Bytes = %d, want 5", b)
	}
}

func TestCache_BytesDecrementOnExpiry(t *testing.T) {
	c := newCache(t)
	c.Set("k", []byte("hello"), -time.Second) // expired immediately
	c.Get("k")                                // triggers removal
	if b := c.Stats().Bytes; b != 0 {
		t.Errorf("Bytes = %d, want 0 after expired entry removed", b)
	}
}

func TestCache_BytesDecrementOnLRUEviction(t *testing.T) {
	c, err := cache.New("test", 2)
	if err != nil {
		t.Fatal(err)
	}
	c.Set("a", []byte("12345"), time.Minute) // 5 bytes
	c.Set("b", []byte("67890"), time.Minute) // 5 bytes — total 10
	c.Set("c", []byte("abc"), time.Minute)   // 3 bytes — evicts "a" (5 bytes), total = 10-5+3 = 8
	if b := c.Stats().Bytes; b != 8 {
		t.Errorf("Bytes = %d, want 8 after LRU eviction", b)
	}
}

func TestCache_BytesUpdatedOnOverwrite(t *testing.T) {
	c := newCache(t)
	c.Set("k", []byte("hello"), time.Minute) // 5 bytes
	c.Set("k", []byte("hi"), time.Minute)    // 2 bytes — overwrites, net = 2
	if b := c.Stats().Bytes; b != 2 {
		t.Errorf("Bytes = %d, want 2 after overwrite", b)
	}
}

func TestCache_Flush(t *testing.T) {
	c := newCache(t)
	c.Set("a", []byte("hello"), time.Minute)
	c.Set("b", []byte("world"), time.Minute)
	c.Flush()

	s := c.Stats()
	if s.Size != 0 {
		t.Errorf("Size = %d, want 0 after Flush", s.Size)
	}
	if s.Bytes != 0 {
		t.Errorf("Bytes = %d, want 0 after Flush", s.Bytes)
	}
	if _, ok := c.Get("a"); ok {
		t.Error("expected miss after Flush, got hit")
	}
}
