package httpapi

import (
	"net/http"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
)

// SystemStatusResponse matches the frontend SystemStatus type.
type SystemStatusResponse struct {
	Status  string             `json:"status"`
	Version string             `json:"version"`
	Uptime  float64            `json:"uptime"`
	Tokens  StatusTokenCount   `json:"tokens"`
	APIKeys StatusAPIKeyCount  `json:"api_keys"`
	Config  SystemConfigStatus `json:"config"`
}

// StatusTokenCount holds token counts for system status.
type StatusTokenCount struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

// StatusAPIKeyCount holds API key counts for system status.
type StatusAPIKeyCount struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

// SystemConfigStatus describes where the active runtime config is coming from.
type SystemConfigStatus struct {
	AppKeySource    string `json:"app_key_source"`
	HasDBOverrides  bool   `json:"has_db_overrides"`
	DBOverrideCount int    `json:"db_override_count"`
}

type systemConfigStore interface {
	Get(key string) (string, error)
	GetAll() (map[string]string, error)
}

type systemConfigInspector func() SystemConfigStatus

// handleSystemStatus returns a handler that reports system status.
func handleSystemStatus(ts TokenStoreInterface, aks APIKeyStoreInterface, startTime time.Time, version string, inspectConfig systemConfigInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := SystemStatusResponse{
			Status:  "healthy",
			Version: version,
			Uptime:  time.Since(startTime).Seconds(),
			Tokens:  StatusTokenCount{},
			APIKeys: StatusAPIKeyCount{},
			Config:  SystemConfigStatus{AppKeySource: "default"},
		}
		if inspectConfig != nil {
			resp.Config = inspectConfig()
		}

		// Get token counts
		if ts != nil {
			tokens, err := ts.ListTokens(r.Context())
			if err == nil {
				resp.Tokens.Total = len(tokens)
				for _, t := range tokens {
					if t.Status == "active" {
						resp.Tokens.Active++
					}
				}
			}
		}

		// Get API key counts
		if aks != nil {
			total, active, _, _, _, err := aks.CountByStatus(r.Context())
			if err == nil {
				resp.APIKeys.Total = total
				resp.APIKeys.Active = active
			}
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}

func buildSystemConfigInspector(cfgProvider func() *config.Config, cfgStore systemConfigStore) systemConfigInspector {
	return func() SystemConfigStatus {
		status := SystemConfigStatus{AppKeySource: "default"}

		if cfgStore != nil {
			if kvs, err := cfgStore.GetAll(); err == nil {
				status.DBOverrideCount = len(kvs)
				status.HasDBOverrides = len(kvs) > 0
			}
			if value, err := cfgStore.Get("app.app_key"); err == nil && value != "" {
				status.AppKeySource = "db"
				return status
			}
		}

		cfg := config.DefaultConfig()
		if cfgProvider != nil {
			if current := cfgProvider(); current != nil {
				cfg = current
			}
		}
		if cfg.App.AppKey != config.DefaultConfig().App.AppKey {
			status.AppKeySource = "config"
		}
		return status
	}
}
