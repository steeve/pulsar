package bittorrent

import "github.com/scakemyer/libtorrent-go"

type ltAlert struct {
	libtorrent.Alert
}

type Alert struct {
	Type     int
	Category int
	What     string
	Message  string
	Pointer  uintptr
}
