package httpapi

import (
	"embed"
	"net/http"
)

//go:embed admin_access.html
var adminUIFS embed.FS

func handleAdminAccessRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := "/admin/access"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}
}

func handleAdminAccessPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := adminUIFS.ReadFile("admin_access.html")
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "server_error", "admin_ui_unavailable",
				"Admin UI is unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}
