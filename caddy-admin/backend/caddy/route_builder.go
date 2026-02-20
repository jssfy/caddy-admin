package caddy

import "encoding/json"

// ServiceConfig describes a dynamically registered service.
type ServiceConfig struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
}

// BuildCaddyRoute generates a Caddy JSON route with @id for a service.
// The route matches the service domain and reverse-proxies to the upstream.
func BuildCaddyRoute(svc ServiceConfig) json.RawMessage {
	route := map[string]any{
		"@id": "svc-" + svc.Name,
		"match": []map[string]any{
			{"host": []string{svc.Domain}},
		},
		"handle": []map[string]any{
			{
				"handler": "subroute",
				"routes": []map[string]any{
					{
						"handle": []map[string]any{
							{
								"handler":   "reverse_proxy",
								"upstreams": []map[string]string{{"dial": svc.Upstream}},
							},
						},
					},
				},
			},
		},
		"terminal": true,
	}
	data, _ := json.Marshal(route)
	return data
}
