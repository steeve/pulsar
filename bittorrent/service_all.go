// +build !arm

package bittorrent

import "github.com/steeve/libtorrent-go"

// Nothing to do on regular devices
func setPlatformSpecificSettings(settings libtorrent.Session_settings) {
}
