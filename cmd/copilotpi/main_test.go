package main

import (
	"testing"

	"github.com/crmmc/copilotpi/internal/config"
)

func TestValidateStartupConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{name: "nil config rejected", cfg: nil, wantErr: true},
		{name: "empty app key rejected", cfg: &config.Config{App: config.AppConfig{AppKey: ""}}, wantErr: true},
		{name: "default app key rejected", cfg: &config.Config{App: config.AppConfig{AppKey: defaultAdminAppKey}}, wantErr: true},
		{name: "custom app key accepted", cfg: &config.Config{App: config.AppConfig{AppKey: "replace-me-with-a-real-secret"}}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStartupConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateStartupConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
