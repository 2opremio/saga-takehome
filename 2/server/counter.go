package server

import (
	"errors"
	"sync/atomic"
)

var OverflowError = errors.New("overflow")

type Counter struct {
	counter atomic.Uint64
}

func NewCounter() *Counter {
	return &Counter{}
}

func (s *Counter) Bump(by uint64) (uint64, error) {
	// TODO: preventively check the overflow, without hurting performance
	result := s.counter.Add(by)
	if result < by {
		return 0, OverflowError
	}
	return result, nil
}
