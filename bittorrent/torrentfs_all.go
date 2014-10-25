// +build !darwin,!freebsd,!dragonfly

package bittorrent

func unlockFile(path string) error {
	return nil
}
