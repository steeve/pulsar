package main

import (
	"os"
	"path/filepath"

	"github.com/steeve/pulsar/config"
)

func Migrate() {
	firstRun := filepath.Join(config.Get().Info.Path, ".firstrun")
	if _, err := os.Stat(firstRun); err == nil {
		return
	}
	file, _ := os.Create(firstRun)
	file.Close()

	log.Info("Preparing for first run")

	// Move ga client id file out of the cache directory
	gaFile := filepath.Join(config.Get().Info.Profile, "cache", "io.steeve.pulsar.ga")
	if _, err := os.Stat(gaFile); err == nil {
		os.Rename(gaFile, filepath.Join(config.Get().Info.Profile, "io.steeve.pulsar.ga"))
	}

	// // Remove the cache
	log.Info("Clearing cache")
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
}
