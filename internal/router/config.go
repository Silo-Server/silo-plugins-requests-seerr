package router

import (
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

// Connection is the per-call Seerr connection: base URL + API key (host chrome)
// plus plugin_config fields.
type Connection struct {
	ID         string
	BaseURL    string
	APIKey     string
	Supports4K bool

	Mapped            bool // requester_mode == "mapped"
	RequireMappedUser bool // mapped + can't map -> fail the request (else admin fallback)
	AutoApprove       bool // grant AUTO_APPROVE(+4K) to mapped users
}

// connectionFromRouter parses a host-supplied RouterConnection. base_url and
// api_key are chrome; the remaining fields come from the config struct.
func connectionFromRouter(c *pluginv1.RouterConnection) Connection {
	conn := Connection{
		ID:      c.GetId(),
		BaseURL: c.GetBaseUrl(),
		APIKey:  c.GetApiKey(),
	}
	if cfg := c.GetConfig(); cfg != nil {
		m := cfg.AsMap()
		if v, ok := m["supports_4k"].(bool); ok {
			conn.Supports4K = v
		}
		if v, ok := m["requester_mode"].(string); ok {
			conn.Mapped = v == "mapped"
		}
		conn.RequireMappedUser, _ = m["require_mapped_user"].(bool)
		conn.AutoApprove, _ = m["auto_approve"].(bool)
	}
	return conn
}
