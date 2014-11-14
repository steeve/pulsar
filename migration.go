package main

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/repository"
)

func Migrate() {
	firstRun := filepath.Join(config.Get().Info.Path, ".firstrun")
	if _, err := os.Stat(firstRun); err == nil {
		return
	}
	file, _ := os.Create(firstRun)
	defer file.Close()

	log.Info("Preparing for first run")

	// Move ga client id file out of the cache directory
	gaFile := filepath.Join(config.Get().Info.Profile, "cache", "io.steeve.pulsar.ga")
	if _, err := os.Stat(gaFile); err == nil {
		os.Rename(gaFile, filepath.Join(config.Get().Info.Profile, "io.steeve.pulsar.ga"))
	}

	gaFile = filepath.Join(config.Get().Info.Profile, "io.steeve.pulsar.ga")
	if file, err := os.Open(gaFile); err == nil {
		if gzReader, err := gzip.NewReader(file); err != nil {
			outFile, _ := os.Create(gaFile + ".gz")
			gzWriter := gzip.NewWriter(outFile)
			file.Seek(0, os.SEEK_SET)
			io.Copy(gzWriter, file)
			gzWriter.Flush()
			gzWriter.Close()
			outFile.Close()
			file.Close()
			os.Rename(gaFile+".gz", gaFile)
		} else {
			gzReader.Close()
		}
	}

	// Remove the cache
	log.Info("Clearing cache")
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))

	log.Info("Creating Pulsar Repository Addon")
	if err := repository.MakePulsarRepositoryAddon(); err != nil {
		log.Error("Unable to create repository addon: %s", err)
	}
}
