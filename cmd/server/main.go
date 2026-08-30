// @title			Breeze API
// @version		1.0
// @description	Self-hosted project management tool API
// @BasePath		/api
package main

import (
	"context"
	"log"

	"ipmanlk/breeze/internal/app"
	"ipmanlk/breeze/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("breeze: invalid configuration: %v", err)
	}

	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("breeze: failed to initialize: %v", err)
	}

	if err := a.Run(context.Background()); err != nil {
		log.Fatalf("breeze: %v", err)
	}
}
