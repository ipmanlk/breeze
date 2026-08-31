// @title			Plume API
// @version		1.0
// @description	Self-hosted project management tool API
// @BasePath		/api
package main

import (
	"context"
	"log"

	"ipmanlk/plume/internal/app"
	"ipmanlk/plume/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("plume: invalid configuration: %v", err)
	}

	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("plume: failed to initialize: %v", err)
	}

	if err := a.Run(context.Background()); err != nil {
		log.Fatalf("plume: %v", err)
	}
}
