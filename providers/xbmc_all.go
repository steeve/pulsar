// +build !arm

package providers

import "time"

func providerTimeout() time.Duration {
	return 10 * time.Second
}
