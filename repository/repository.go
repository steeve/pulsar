package repository

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/util"
	"github.com/i96751414/pulsar/xbmc"
)

func copyFile(from string, to string) error {
	input, err := os.Open(from)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(to)
	if err != nil {
		return err
	}
	defer output.Close()
	io.Copy(output, input)
	return nil
}

func MakePulsarRepositoryAddon() error {
	addonId := "repository.pulsar"
	addonName := "Pulsar Repository"

	pulsarHost := fmt.Sprintf("http://localhost:%d", config.ListenPort)
	addon := &xbmc.Addon{
		Id:           addonId,
		Name:         addonName,
		Version:      util.Version,
		ProviderName: config.Get().Info.Author,
		Extensions: []*xbmc.AddonExtension{
			&xbmc.AddonExtension{
				Point: "xbmc.addon.repository",
				Name:  addonName,
				Info: &xbmc.AddonRepositoryInfo{
					Text:       pulsarHost + "/repository/steeve/plugin.video.pulsar/addons.xml",
					Compressed: false,
				},
				Checksum: pulsarHost + "/repository/steeve/plugin.video.pulsar/addons.xml.md5",
				Datadir: &xbmc.AddonRepositoryDataDir{
					Text: pulsarHost + "/repository/steeve/",
					Zip:  true,
				},
			},
			&xbmc.AddonExtension{
				Point: "xbmc.addon.metadata",
				Summaries: []*xbmc.AddonText{
					&xbmc.AddonText{"Virtual repository for Pulsar Updates", "en"},
				},
				Platform: "all",
			},
		},
	}

	addonPath := filepath.Clean(filepath.Join(config.Get().Info.Path, "..", addonId))
	if err := os.MkdirAll(addonPath, 0777); err != nil {
		return err
	}

	if err := copyFile(filepath.Join(config.Get().Info.Path, "icon.png"), filepath.Join(addonPath, "icon.png")); err != nil {
		return err
	}

	addonXmlFile, err := os.Create(filepath.Join(addonPath, "addon.xml"))
	if err != nil {
		return err
	}
	defer addonXmlFile.Close()
	return xml.NewEncoder(addonXmlFile).Encode(addon)
}
