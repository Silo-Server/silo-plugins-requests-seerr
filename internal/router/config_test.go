package router

import (
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestConnectionFromRouterParsesRequesterConfig(t *testing.T) {
	s, _ := structpb.NewStruct(map[string]any{
		"supports_4k": true, "requester_mode": "mapped", "require_mapped_user": true,
		"auto_approve": true,
	})
	conn := connectionFromRouter(&pluginv1.RouterConnection{Id: "c1", BaseUrl: "http://x", ApiKey: "k", Config: s})
	if !conn.Mapped || !conn.Supports4K || !conn.RequireMappedUser || !conn.AutoApprove {
		t.Fatalf("parsed wrong: %+v", conn)
	}
}
