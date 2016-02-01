package main

import (
	"os"
	"path/filepath"

	"github.com/scakemyer/quasar/xbmc"
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

	log.Info("Preparing for first run")

	// Remove the cache
	log.Info("Clearing cache")
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))

	log.Info("Creating Quasar Repository Addon")
	if err := repository.MakeQuasarRepositoryAddon(); err != nil {
		log.Error("Unable to create repository addon: %s", err)
	} else {
		log.Info("Updating Kodi Addon Repositories")
		xbmc.UpdateAddonRepos()
	}
}
