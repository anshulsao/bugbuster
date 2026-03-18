package bugbuster

import "embed"

// Assets embeds all runtime files needed by bugbuster.
//
//go:embed docker-compose.yml docker-compose.observability.yml all:scenarios all:services all:observability all:seed-data all:loadgen
var Assets embed.FS
