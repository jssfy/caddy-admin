package handlers

import (
	"caddy-admin/caddy"
	"caddy-admin/store"
	"encoding/json"
	"log"
	"net/http"
)

// ServicesHandler handles dynamic service registration API.
type ServicesHandler struct {
	caddyClient *caddy.Client
	fileStore   *store.FileStore
}

// NewServicesHandler creates a new ServicesHandler.
func NewServicesHandler(client *caddy.Client, fs *store.FileStore) *ServicesHandler {
	return &ServicesHandler{caddyClient: client, fileStore: fs}
}

// Register handles POST /api/services
func (h *ServicesHandler) Register(w http.ResponseWriter, r *http.Request) {
	var svc caddy.ServiceConfig
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if svc.Name == "" || svc.Domain == "" || svc.Upstream == "" {
		writeError(w, http.StatusBadRequest, "name, domain, and upstream are required")
		return
	}

	if err := h.caddyClient.UpsertRoute(svc); err != nil {
		writeError(w, http.StatusBadGateway, "caddy upsert failed: "+err.Error())
		return
	}

	if err := h.fileStore.Upsert(svc); err != nil {
		writeError(w, http.StatusInternalServerError, "persist failed: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"registered": true,
		"name":       svc.Name,
		"domain":     svc.Domain,
		"upstream":   svc.Upstream,
	})
}

// Deregister handles DELETE /api/services/{name}
func (h *ServicesHandler) Deregister(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	if err := h.caddyClient.RemoveRoute(name); err != nil {
		writeError(w, http.StatusBadGateway, "caddy remove failed: "+err.Error())
		return
	}

	if err := h.fileStore.Delete(name); err != nil {
		writeError(w, http.StatusInternalServerError, "persist failed: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"deleted": true, "name": name})
}

// List handles GET /api/services
func (h *ServicesHandler) List(w http.ResponseWriter, r *http.Request) {
	services, err := h.fileStore.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load failed: "+err.Error())
		return
	}
	writeJSON(w, map[string]any{"services": services, "total": len(services)})
}

// Sync handles POST /api/services/sync — manually trigger syncToCaddy
func (h *ServicesHandler) Sync(w http.ResponseWriter, r *http.Request) {
	services, err := h.fileStore.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load failed: "+err.Error())
		return
	}

	synced := 0
	var errors []string
	for _, svc := range services {
		if err := h.caddyClient.UpsertRoute(svc); err != nil {
			errors = append(errors, svc.Name+": "+err.Error())
		} else {
			synced++
		}
	}

	if len(errors) > 0 {
		log.Printf("sync partial failure: %v", errors)
	}

	writeJSON(w, map[string]any{
		"synced": synced,
		"total":  len(services),
		"errors": errors,
	})
}
