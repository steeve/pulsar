package xbmc

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("xbmc")

const (
	LogDebug = iota
	LogInfo
	LogNotice
	LogWarning
	LogError
	LogSevere
	LogFatal
	LogNone
)

type LogBackend struct{}

func Log(args ...interface{}) {
	executeJSONRPCEx("Log", nil, args)
}

func NewLogBackend() *LogBackend {
	return &LogBackend{}
}

func (b *LogBackend) Log(level logging.Level, calldepth int, rec *logging.Record) error {
	line := rec.Formatted(calldepth + 1)
	switch level {
	case logging.CRITICAL:
		Log(line, LogSevere)
	case logging.ERROR:
		Log(line, LogError)
	case logging.WARNING:
		Log(line, LogWarning)
	case logging.NOTICE:
		Log(line, LogNotice)
	case logging.INFO:
		Log(line, LogInfo)
	case logging.DEBUG:
		Log(line, LogDebug)
	default:
	}
	return nil
}
