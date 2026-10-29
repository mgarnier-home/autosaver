package main

import (
	"time"

	"github.com/mgarnier-home/utils/logger"

	"mgarnier11.fr/go/autosaver/backup"
	"mgarnier11.fr/go/autosaver/config"
	"mgarnier11.fr/go/autosaver/server"
	"mgarnier11.fr/go/autosaver/stats"
)

func main() {
	// Avant le logger : le .env chargé ici peut définir LOG_LEVEL.
	config.Load()
	logger.InitAppLogger("GO-AUTOSAVER")
	backup.Stats = stats.Open(config.Config.StatsFilePath, time.Now)

	api := server.NewServer(config.Config.ServerPort)
	api.Start()
}
