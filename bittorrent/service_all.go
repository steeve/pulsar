// +build !arm

package bittorrent

import "github.com/i96751414/libtorrent-go"

// Nothing to do on regular devices
func setPlatformSpecificSettings(settings libtorrent.Session_settings) {
}
