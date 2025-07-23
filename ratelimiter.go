package main

import "time"

type RateLimiter struct {
	tokens chan struct{}
	ticker *time.Ticker
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		tokens: make(chan struct{}, 5),
		ticker: time.NewTicker(time.Second),
	}

	// Add initial tokens
	for i := 0; i < 5; i++ {
		rl.tokens <- struct{}{}
	}

	go func() {
		for range rl.ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

func (rl *RateLimiter) Close() {
	rl.ticker.Stop()
}
