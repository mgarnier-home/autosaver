package main

import (
	"github.com/mgarnier-home/utils/logger"

	"mgarnier11.fr/go/autosaver/config"
	"mgarnier11.fr/go/autosaver/server"
)

func main() {
	// Avant le logger : le .env chargé ici peut définir LOG_LEVEL.
	config.Load()
	logger.InitAppLogger("GO-AUTOSAVER")

	api := server.NewServer(config.Config.ServerPort)
	api.Start()
}
