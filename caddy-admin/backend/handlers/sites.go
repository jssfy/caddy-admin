package handlers

import (
	"caddy-admin/caddy"
	"net/http"
	"strings"
)

type SitesHandler struct {
	caddyClient *caddy.Client
}

func NewSitesHandler(client *caddy.Client) *SitesHandler {
	return &SitesHandler{caddyClient: client}
}

// ListSites handles GET /api/sites
func (h *SitesHandler) ListSites(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.caddyClient.GetConfig()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "cannot reach caddy: "+err.Error())
		return
	}

	sites := caddy.ParseSites(cfg)
	writeJSON(w, map[string]any{
		"sites": sites,
		"total": len(sites),
	})
}

// GetSite handles GET /api/sites/{domain}
func (h *SitesHandler) GetSite(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain required")
		return
	}

	cfg, err := h.caddyClient.GetConfig()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "cannot reach caddy: "+err.Error())
		return
	}

	sites := caddy.ParseSites(cfg)
	for _, s := range sites {
		if strings.EqualFold(s.Domain, domain) {
			writeJSON(w, s)
			return
		}
	}
	writeError(w, http.StatusNotFound, "site not found: "+domain)
}

// Status handles GET /api/status
func (h *SitesHandler) Status(w http.ResponseWriter, r *http.Request) {
	running := h.caddyClient.IsRunning()
	writeJSON(w, map[string]any{
		"caddy": running,
	})
}
