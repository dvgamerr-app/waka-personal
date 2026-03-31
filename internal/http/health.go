package http

import "sync/atomic"

type Checker struct {
	ready atomic.Bool
}

func (c *Checker) SetReady(v bool) {
	c.ready.Store(v)
}

func (c *Checker) IsReady() bool {
	return c.ready.Load()
}
