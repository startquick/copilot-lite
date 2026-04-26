package config

import (
	"strconv"
	"sync"
	"testing"
)

func TestRuntimeCloneIsolation(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			FilterTags: []string{"copilot-tag"},
		},
		Retry: RetryConfig{
			ResetSessionStatusCodes: []int{401},
			CoolingStatusCodes:      []int{429},
		},
		Token: TokenConfig{
			BasicModels: []string{"copilot-free"},
			SuperModels: []string{"copilot-premium"},
		},
	}

	runtime := NewRuntime(cfg)
	snapshot := runtime.Snapshot()
	snapshot.App.FilterTags[0] = "changed"
	snapshot.Retry.ResetSessionStatusCodes[0] = 500
	snapshot.Token.BasicModels[0] = "changed-model"

	current := runtime.Get()
	if current.App.FilterTags[0] != "copilot-tag" {
		t.Fatalf("runtime config mutated filter tags: %v", current.App.FilterTags)
	}
	if current.Retry.ResetSessionStatusCodes[0] != 401 {
		t.Fatalf("runtime config mutated retry config: %v", current.Retry.ResetSessionStatusCodes)
	}
	if current.Token.BasicModels[0] != "copilot-free" {
		t.Fatalf("runtime config mutated token config: %v", current.Token.BasicModels)
	}
}

func TestApplyDBOverrides_TrimsModels(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ApplyDBOverrides(map[string]string{
		"token.basic_models": " copilot-free , copilot-basic , ",
		"token.super_models": " copilot-premium ",
	})

	if len(cfg.Token.BasicModels) != 2 || cfg.Token.BasicModels[0] != "copilot-free" || cfg.Token.BasicModels[1] != "copilot-basic" {
		t.Fatalf("basic models not trimmed correctly: %#v", cfg.Token.BasicModels)
	}
	if len(cfg.Token.SuperModels) != 1 || cfg.Token.SuperModels[0] != "copilot-premium" {
		t.Fatalf("super models not trimmed correctly: %#v", cfg.Token.SuperModels)
	}
}

func TestRuntimeUpdateSerializesConcurrentWriters(t *testing.T) {
	runtime := NewRuntime(&Config{})

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := runtime.Update(func(cfg *Config) error {
				_ = strconv.Itoa(i)
				cfg.Token.FailThreshold++
				return nil
			})
			if err != nil {
				t.Errorf("Update() error = %v", err)
			}
		}()
	}
	wg.Wait()

	current := runtime.Get()
	if current.Token.FailThreshold != 32 {
		t.Fatalf("expected all concurrent updates to be preserved, got fail_threshold=%d", current.Token.FailThreshold)
	}
}
