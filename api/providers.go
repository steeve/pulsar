package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

type Addon struct {
	ID      string
	Name    string
	Version string
	Enabled bool
}

func getProviders() []Addon {
	list := make([]Addon, 0)
	for _, addon := range xbmc.GetAddons("xbmc.python.script", "executable", "all", []string{"name", "version", "enabled"}).Addons {
		if strings.HasPrefix(addon.ID, "script.quasar.") {
			list = append(list, Addon{
				ID: addon.ID,
				Name: addon.Name,
				Enabled: addon.Enabled,
				Version: addon.Version,
			})
		}
	}
	return list
}

func ProviderList(ctx *gin.Context) {
	providers := getProviders()

	items := make(xbmc.ListItems, 0, len(providers))
	for _, provider := range providers {
		status := "[COLOR FF009900]OK[/COLOR]"
		failures := xbmc.AddonCheck(provider.ID)
		if failures > 0 {
			status = "[COLOR FF999900]WARN[/COLOR]"
		}

		enabled := "[COLOR FF009900]Enabled[/COLOR]"
		if provider.Enabled == false {
			enabled = "[COLOR FF990000]Disabled[/COLOR]"
		}

		item := &xbmc.ListItem{
			Label:      fmt.Sprintf("%s - %s - %s %s", status, enabled, provider.Name, provider.Version),
			Path:       UrlForXBMC("/provider/%s/check", provider.ID),
			IsPlayable: false,
		}
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30242]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/check", provider.ID))},
			[]string{"LOCALIZE[30240]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/enable", provider.ID))},
			[]string{"LOCALIZE[30241]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/disable", provider.ID))},
		}
		if provider.Enabled {
			item.ContextMenu = append(item.ContextMenu, []string{
				"LOCALIZE[30244]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/settings", provider.ID)),
			})
		}
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("", items))
}

func ProviderSettings(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.AddonSettings(addonId)
	ctx.String(200, "")
}

func ProviderCheck(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	failures := xbmc.AddonCheck(addonId)
	translated := xbmc.GetLocalizedString(30243)
	xbmc.Notify("Quasar", fmt.Sprintf("%s: %d", translated, failures), config.AddonIcon())
  ctx.String(200, "")
}

func ProviderFailure(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.AddonFailure(addonId)
	ctx.String(200, "")
}

func ProviderEnable(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.SetAddonEnabled(addonId, true)
	path := xbmc.InfoLabel("Container.FolderPath")
	if path == "plugin://plugin.video.quasar/provider/" {
		xbmc.Refresh()
	}
  ctx.String(200, "")
}

func ProviderDisable(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.SetAddonEnabled(addonId, false)
	path := xbmc.InfoLabel("Container.FolderPath")
	if path == "plugin://plugin.video.quasar/provider/" {
		xbmc.Refresh()
	}
  ctx.String(200, "")
}
