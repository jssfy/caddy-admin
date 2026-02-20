package main

import (
	"caddy-admin/caddy"
	"caddy-admin/handlers"
	"caddy-admin/store"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	adminAddr := getEnv("CADDY_ADMIN_ADDR", "localhost:2019")
	certStore := getEnv("CADDY_CERT_STORE", "/data/caddy")
	externalCertDir := getEnv("EXTERNAL_CERT_DIR", "")
	listenAddr := getEnv("LISTEN_ADDR", ":8090")
	servicesFile := getEnv("SERVICES_FILE", "/app/data/services.json")

	caddyClient := caddy.NewClient(adminAddr)
	fileStore := store.NewFileStore(servicesFile)

	sitesHandler := handlers.NewSitesHandler(caddyClient)
	certsHandler := handlers.NewCertsHandler(certStore, externalCertDir)
	servicesHandler := handlers.NewServicesHandler(caddyClient, fileStore)

	mux := http.NewServeMux()

	// CORS middleware wrapper
	handler := withCORS(mux)

	// Existing routes
	mux.HandleFunc("GET /api/status", sitesHandler.Status)
	mux.HandleFunc("GET /api/sites", sitesHandler.ListSites)
	mux.HandleFunc("GET /api/sites/{domain}", sitesHandler.GetSite)
	mux.HandleFunc("GET /api/certs", certsHandler.ListCerts)

	// Service registration routes
	mux.HandleFunc("GET /api/services", servicesHandler.List)
	mux.HandleFunc("POST /api/services", servicesHandler.Register)
	mux.HandleFunc("DELETE /api/services/{name}", servicesHandler.Deregister)
	mux.HandleFunc("POST /api/services/sync", servicesHandler.Sync)

	// Sync persisted services to Caddy on startup
	go syncToCaddy(caddyClient, fileStore)

	log.Printf("caddy-admin API listening on %s (caddy at %s)", listenAddr, adminAddr)
	if err := http.ListenAndServe(listenAddr, handler); err != nil {
		log.Fatal(err)
	}
}

// syncToCaddy waits for Caddy to become ready, then replays all persisted services.
func syncToCaddy(client *caddy.Client, fs *store.FileStore) {
	// Wait for Caddy to be ready
	for i := 0; i < 15; i++ {
		if client.IsRunning() {
			break
		}
		log.Printf("sync: waiting for caddy... (%d/15)", i+1)
		time.Sleep(2 * time.Second)
	}

	if !client.IsRunning() {
		log.Println("sync: caddy not ready after 30s, skipping")
		return
	}

	services, err := fs.Load()
	if err != nil {
		log.Printf("sync: load services failed: %v", err)
		return
	}

	if len(services) == 0 {
		log.Println("sync: no persisted services")
		return
	}

	synced := 0
	for _, svc := range services {
		if err := client.UpsertRoute(svc); err != nil {
			log.Printf("sync: failed to upsert %s: %v", svc.Name, err)
		} else {
			synced++
		}
	}
	log.Printf("sync: restored %d/%d services to caddy", synced, len(services))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
