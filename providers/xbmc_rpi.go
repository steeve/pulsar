// +build arm

package providers

import (
	"runtime"
	"time"
)

func providerTimeout() time.Duration {
	if runtime.NumCPU() == 1 {
		return 30 * time.Second
	}
	return 20 * time.Second
}
