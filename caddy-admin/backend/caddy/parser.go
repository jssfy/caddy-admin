package caddy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"crypto/x509"
	"encoding/pem"
)

// SiteInfo is the extracted info for one virtual host
type SiteInfo struct {
	Domain   string            `json:"domain"`
	Type     string            `json:"type"`     // "static" | "proxy" | "unknown"
	Root     string            `json:"root,omitempty"`
	Upstream string            `json:"upstream,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	HasTLS   bool              `json:"hasTLS"`
}

// CertInfo is the extracted info for one TLS certificate
type CertInfo struct {
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"notBefore"`
	NotAfter  time.Time `json:"notAfter"`
	DaysLeft  int       `json:"daysLeft"`
	IsExpired bool      `json:"isExpired"`
	Source    string    `json:"source"` // "letsencrypt" | "local" | "zerossl" | "unknown"
}

// ParseSites extracts all virtual hosts from the Caddy config
func ParseSites(cfg *CaddyConfig) []SiteInfo {
	httpRaw, ok := cfg.Apps["http"]
	if !ok {
		return nil
	}

	var httpApp HTTPApp
	if err := json.Unmarshal(httpRaw, &httpApp); err != nil {
		return nil
	}

	// Collect domains managed by TLS automation
	tlsDomains := parseTLSDomains(cfg)

	var sites []SiteInfo
	for _, server := range httpApp.Servers {
		for _, route := range server.Routes {
			for _, match := range route.Match {
				for _, host := range match.Host {
					site := SiteInfo{
						Domain: host,
						HasTLS: tlsDomains[host],
					}
					extractHandlerInfo(&site, route.Handle)
					sites = append(sites, site)
				}
			}
		}
	}
	return sites
}

// parseTLSDomains returns a set of domains with TLS automation
func parseTLSDomains(cfg *CaddyConfig) map[string]bool {
	result := make(map[string]bool)
	tlsRaw, ok := cfg.Apps["tls"]
	if !ok {
		return result
	}
	var tlsApp TLSApp
	if err := json.Unmarshal(tlsRaw, &tlsApp); err != nil {
		return result
	}
	if tlsApp.Automation == nil {
		return result
	}
	for _, policy := range tlsApp.Automation.Policies {
		for _, subj := range policy.Subjects {
			result[subj] = true
		}
	}
	return result
}

// extractHandlerInfo walks through handle entries to find site type
func extractHandlerInfo(site *SiteInfo, handles []json.RawMessage) {
	for _, raw := range handles {
		var h Handler
		if err := json.Unmarshal(raw, &h); err != nil {
			continue
		}
		switch h.Handler {
		case "subroute":
			// Recurse into subroute routes
			for _, r := range h.Routes {
				extractHandlerInfo(site, r.Handle)
			}
		case "file_server":
			site.Type = "static"
			if h.Root != "" {
				site.Root = h.Root
			}
		case "reverse_proxy":
			site.Type = "proxy"
			if len(h.Upstreams) > 0 {
				site.Upstream = h.Upstreams[0].Dial
			}
		case "headers":
			if h.Response != nil && len(h.Response.Set) > 0 {
				if site.Headers == nil {
					site.Headers = make(map[string]string)
				}
				for k, vals := range h.Response.Set {
					if len(vals) > 0 {
						site.Headers[k] = vals[0]
					}
				}
			}
		}
	}
	if site.Type == "" {
		site.Type = "unknown"
	}
}

// ReadCerts scans the Caddy certificate storage directory and returns cert info
func ReadCerts(certStorePath string) []CertInfo {
	var certs []CertInfo

	// Caddy stores certs under: <certStorePath>/certificates/<issuer>/<domain>/<domain>.crt
	certsBase := filepath.Join(certStorePath, "certificates")
	issuerDirs, err := os.ReadDir(certsBase)
	if err != nil {
		return certs
	}

	for _, issuerEntry := range issuerDirs {
		if !issuerEntry.IsDir() {
			continue
		}
		issuerDir := filepath.Join(certsBase, issuerEntry.Name())
		domainDirs, err := os.ReadDir(issuerDir)
		if err != nil {
			continue
		}

		for _, domainEntry := range domainDirs {
			if !domainEntry.IsDir() {
				continue
			}
			domain := domainEntry.Name()
			certFile := filepath.Join(issuerDir, domain, domain+".crt")
			cert, err := parseCertFile(certFile)
			if err != nil {
				continue
			}

			source := classifyIssuerDir(issuerEntry.Name())
			daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)

			certs = append(certs, CertInfo{
				Domain:    domain,
				Issuer:    cert.Issuer.CommonName,
				NotBefore: cert.NotBefore,
				NotAfter:  cert.NotAfter,
				DaysLeft:  daysLeft,
				IsExpired: daysLeft < 0,
				Source:    source,
			})
		}
	}
	return certs
}

// ReadExternalCerts reads PEM certificate files from a flat directory (e.g. acme.sh install-cert output)
func ReadExternalCerts(dir string) []CertInfo {
	var certs []CertInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return certs
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".cer") {
			continue
		}
		// Skip key files
		if strings.Contains(name, "key") {
			continue
		}

		certPath := filepath.Join(dir, name)
		cert, err := parseCertFile(certPath)
		if err != nil {
			continue
		}

		daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
		domain := cert.Subject.CommonName
		if len(cert.DNSNames) > 0 {
			domain = cert.DNSNames[0]
		}

		certs = append(certs, CertInfo{
			Domain:    domain,
			Issuer:    cert.Issuer.CommonName,
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
			DaysLeft:  daysLeft,
			IsExpired: daysLeft < 0,
			Source:    classifyIssuer(cert.Issuer.CommonName),
		})
	}
	return certs
}

// classifyIssuer determines the cert source from the issuer CN
func classifyIssuer(issuerCN string) string {
	lower := strings.ToLower(issuerCN)
	switch {
	case strings.Contains(lower, "let's encrypt") || strings.Contains(lower, "letsencrypt"):
		return "letsencrypt"
	case strings.Contains(lower, "zerossl"):
		return "zerossl"
	default:
		return "external"
	}
}

func parseCertFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, os.ErrInvalid
	}
	return x509.ParseCertificate(block.Bytes)
}

func classifyIssuerDir(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "letsencrypt"):
		return "letsencrypt"
	case strings.Contains(lower, "zerossl"):
		return "zerossl"
	case strings.Contains(lower, "local"):
		return "local"
	default:
		return "unknown"
	}
}
