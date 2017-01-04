package main

import (
	"os"
	"path/filepath"

	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/repository"
)

func Migrate() {
	firstRun := filepath.Join(config.Get().Info.Path, ".firstrun")
	if _, err := os.Stat(firstRun); err == nil {
		return
	}
	file, _ := os.Create(firstRun)
	defer file.Close()

	log.Info("Preparing for first run...")

	log.Info("Creating Quasar repository add-on")
	if err := repository.MakeQuasarRepositoryAddon(); err != nil {
		log.Errorf("Unable to create repository add-on: %s", err)
	}
}
