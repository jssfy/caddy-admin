package handlers

import (
	"caddy-admin/caddy"
	"net/http"
	"os"
)

type CertsHandler struct {
	certStorePath   string
	externalCertDir string
}

func NewCertsHandler(certStorePath, externalCertDir string) *CertsHandler {
	if certStorePath == "" {
		certStorePath = "/data/caddy"
	}
	return &CertsHandler{
		certStorePath:   certStorePath,
		externalCertDir: externalCertDir,
	}
}

// ListCerts handles GET /api/certs
func (h *CertsHandler) ListCerts(w http.ResponseWriter, r *http.Request) {
	var certs []caddy.CertInfo

	// 1. Caddy 内部自动管理的证书（/data/caddy/certificates/...）
	if _, err := os.Stat(h.certStorePath); err == nil {
		certs = append(certs, caddy.ReadCerts(h.certStorePath)...)
	}

	// 2. 外部 acme.sh 签发的证书（~/certs/yeanhua.asia/ 挂载到容器）
	if h.externalCertDir != "" {
		certs = append(certs, caddy.ReadExternalCerts(h.externalCertDir)...)
	}

	writeJSON(w, map[string]any{
		"certs": certs,
		"total": len(certs),
	})
}
