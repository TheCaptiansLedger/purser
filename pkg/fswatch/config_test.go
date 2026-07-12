package fswatch

import (
	"testing"
	"time"
)

func TestConfig_DefaultIsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig() is invalid: %v", err)
	}
}

func TestConfig_Validate(t *testing.T) {
	base := DefaultConfig()

	tests := []struct {
		name    string
		mutate  func(c Config) Config
		wantErr bool
	}{
		{"negative coalesce depth", func(c Config) Config { c.CoalesceDepth = -1; return c }, true},
		{"zero settle window", func(c Config) Config { c.SettleWindow = 0; return c }, true},
		{"negative settle window", func(c Config) Config { c.SettleWindow = -time.Second; return c }, true},
		{"max wait less than settle window", func(c Config) Config { c.MaxWait = c.SettleWindow - time.Millisecond; return c }, true},
		{"negative event buffer", func(c Config) Config { c.EventBuffer = -1; return c }, true},
		{"max wait equal to settle window is fine", func(c Config) Config { c.MaxWait = c.SettleWindow; return c }, false},
		{"zero event buffer is fine (unbuffered)", func(c Config) Config { c.EventBuffer = 0; return c }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
