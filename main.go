// Command silo-plugins-requests-seerr implements the Silo request_router.v1
// capability backed by a Seerr (Overseerr/Jellyseerr) instance.
package main

import (
	_ "embed"

	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"

	"github.com/Silo-Server/silo-plugins-requests-seerr/internal/router"
)

var version string

//go:embed manifest.json
var manifestJSON []byte

func main() {
	runtime.ServeManifest(manifestJSON, version, runtime.CapabilityServers{
		RequestRouter: router.New(),
	})
}
