package main

import (
	"caddy-admin/caddy"
	"caddy-admin/handlers"
	"log"
	"net/http"
	"os"
)

func main() {
	adminAddr := getEnv("CADDY_ADMIN_ADDR", "localhost:2019")
	certStore := getEnv("CADDY_CERT_STORE", "/data/caddy")
	externalCertDir := getEnv("EXTERNAL_CERT_DIR", "")
	listenAddr := getEnv("LISTEN_ADDR", ":8090")

	caddyClient := caddy.NewClient(adminAddr)
	sitesHandler := handlers.NewSitesHandler(caddyClient)
	certsHandler := handlers.NewCertsHandler(certStore, externalCertDir)

	mux := http.NewServeMux()

	// CORS middleware wrapper
	handler := withCORS(mux)

	// Routes
	mux.HandleFunc("GET /api/status", sitesHandler.Status)
	mux.HandleFunc("GET /api/sites", sitesHandler.ListSites)
	mux.HandleFunc("GET /api/sites/{domain}", sitesHandler.GetSite)
	mux.HandleFunc("GET /api/certs", certsHandler.ListCerts)

	log.Printf("caddy-admin API listening on %s (caddy at %s)", listenAddr, adminAddr)
	if err := http.ListenAndServe(listenAddr, handler); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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
