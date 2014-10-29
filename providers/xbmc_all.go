// +build !arm

package providers

import "time"

func providerTimeout() time.Duration {
	return 4 * time.Second
}
