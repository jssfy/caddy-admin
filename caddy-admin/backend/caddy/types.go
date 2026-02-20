package caddy

import "encoding/json"

// CaddyConfig is the root config returned by GET /config/
type CaddyConfig struct {
	Apps map[string]json.RawMessage `json:"apps"`
}

// HTTPApp represents the http app config
type HTTPApp struct {
	Servers map[string]HTTPServer `json:"servers"`
}

// HTTPServer is a single HTTP server (srv0, srv1, ...)
type HTTPServer struct {
	Listen []string    `json:"listen"`
	Routes []HTTPRoute `json:"routes"`
}

// HTTPRoute is one route entry (match + handle)
type HTTPRoute struct {
	Match    []MatchRule       `json:"match"`
	Handle   []json.RawMessage `json:"handle"`
	Terminal bool              `json:"terminal"`
}

// MatchRule contains host/path matching conditions
type MatchRule struct {
	Host []string `json:"host"`
	Path []string `json:"path"`
}

// Handler is decoded by "handler" field
type Handler struct {
	Handler   string          `json:"handler"`
	// subroute
	Routes    []HTTPRoute     `json:"routes,omitempty"`
	// file_server
	Root      string          `json:"root,omitempty"`
	// reverse_proxy
	Upstreams []Upstream      `json:"upstreams,omitempty"`
	// headers
	Response  *HeadersOps     `json:"response,omitempty"`
	// encode
	Encodings map[string]interface{} `json:"encodings,omitempty"`
}

// Upstream is a reverse_proxy upstream
type Upstream struct {
	Dial string `json:"dial"`
}

// HeadersOps is the headers handler response config
type HeadersOps struct {
	Set    map[string][]string `json:"set"`
	Add    map[string][]string `json:"add"`
	Delete []string            `json:"delete"`
}

// TLSApp represents the tls app config
type TLSApp struct {
	Automation *TLSAutomation `json:"automation,omitempty"`
}

// TLSAutomation holds ACME automation policies
type TLSAutomation struct {
	Policies []TLSPolicy `json:"policies"`
}

// TLSPolicy holds a list of subjects to automate
type TLSPolicy struct {
	Subjects []string `json:"subjects"`
	Issuers  []json.RawMessage `json:"issuers,omitempty"`
}
