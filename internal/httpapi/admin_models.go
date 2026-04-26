package httpapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/crmmc/copilotpi/internal/config"
	tkn "github.com/crmmc/copilotpi/internal/token"
)

// ModelCatalogEntry describes one model available in this proxy.
type ModelCatalogEntry struct {
	// ID is the bare model name (e.g. "grok-3-mini").
	ID string `json:"id"`
	// APIID is the fully-qualified model ID to use in chat requests (e.g. "grok/grok-3-mini").
	APIID string `json:"api_id"`
	// Pools lists which pools the model belongs to (e.g. ["ssoBasic", "ssoSuper"]).
	Pools []string `json:"pools"`
	// CostMultiplier is the quota cost factor parsed from the "#N" suffix in config.
	CostMultiplier int `json:"cost_multiplier"`
	// Capabilities lists the capabilities of this model.
	Capabilities []string `json:"capabilities"`
}

// exactCapabilities maps specific model IDs (no prefix) to their capability lists.
// These take priority over the pattern-based rules below.
var exactCapabilities = map[string][]string{
	"grok-imagine-1.0":      {"image-gen"},
	"grok-imagine-1.0-fast": {"image-gen"},
	"grok-imagine-1.0-edit": {"image-edit", "vision"},
	"grok-imagine-1.0-video": {"video-gen"},
}

// capabilitiesFor returns the capability set for a given model name.
// It first checks the exact lookup table, then applies pattern-based rules.
// All text-capable chat models support: vision (image_url blocks), tools, and
// optionally thinking (reasoning_effort control).
func capabilitiesFor(name string) []string {
	if caps, ok := exactCapabilities[name]; ok {
		return caps
	}

	// Pattern-based detection for chat models.
	caps := []string{"chat", "vision", "tools"}
	if strings.Contains(name, "thinking") {
		caps = append(caps, "thinking")
	}
	return caps
}

// handleAdminModels returns a handler that lists all configured models with their
// pools, cost multipliers, capabilities, and the API model ID used in requests.
func handleAdminModels(runtime *config.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := runtime.Get()
		if cfg == nil {
			WriteJSON(w, http.StatusOK, []ModelCatalogEntry{})
			return
		}

		type modelData struct {
			cost  int
			pools map[string]struct{}
		}
		models := make(map[string]*modelData)

		collect := func(entries []string, pool string) {
			for _, entry := range entries {
				name, cost := tkn.ParseModelEntry(entry)
				if name == "" {
					continue
				}
				md, ok := models[name]
				if !ok {
					md = &modelData{cost: cost, pools: make(map[string]struct{})}
					models[name] = md
				}
				md.pools[pool] = struct{}{}
				// Cost from the first pool that specifies it wins; subsequent same entries overwrite only if > 1.
				if md.cost == 1 && cost > 1 {
					md.cost = cost
				}
			}
		}

		collect(cfg.Token.BasicModels, "ssoBasic")
		collect(cfg.Token.SuperModels, "ssoSuper")

		entries := make([]ModelCatalogEntry, 0, len(models))
		for name, md := range models {
			pools := make([]string, 0, len(md.pools))
			for p := range md.pools {
				pools = append(pools, p)
			}
			sort.Strings(pools)

			entries = append(entries, ModelCatalogEntry{
				ID:             name,
				APIID:          "grok/" + name,
				Pools:          pools,
				CostMultiplier: md.cost,
				Capabilities:   capabilitiesFor(name),
			})
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ID < entries[j].ID
		})

		WriteJSON(w, http.StatusOK, entries)
	}
}
