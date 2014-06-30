package util

import "time"

type RateLimiter struct {
	rateTicker   *time.Ticker
	rateLimiter  chan bool
	parallelChan chan bool

	BurstRate     int
	BurstTimeSpan time.Duration
	ParallelCount int
}

func NewRateLimiter(burstRate int, burstTimeSpan time.Duration, parallelCount int) *RateLimiter {
	limiter := &RateLimiter{
		rateTicker:   time.NewTicker(burstTimeSpan),
		rateLimiter:  make(chan bool, burstRate),
		parallelChan: make(chan bool, parallelCount),

		BurstRate:     burstRate,
		BurstTimeSpan: burstTimeSpan,
		ParallelCount: parallelCount,
	}
	go func() {
		for _ = range limiter.rateTicker.C {
			limiter.Reset()
		}
	}()
	return limiter
}

func (rl *RateLimiter) Enter() {
	rl.parallelChan <- true
	rl.rateLimiter <- true
}

func (rl *RateLimiter) Leave() {
	<-rl.parallelChan
}

func (rl *RateLimiter) Call(f func()) {
	rl.Enter()
	defer rl.Leave()
	f()
}

func (rl *RateLimiter) Reset() {
outer:
	for i := 0; i < rl.BurstRate; i++ {
		select {
		case <-rl.rateLimiter:
		default:
			break outer
		}
	}
}

func (rl *RateLimiter) Close() {
	rl.rateTicker.Stop()
}
