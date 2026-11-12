package check

import (
	"lowlang/internal/diag"
	"lowlang/internal/types"
)

type phase int

const (
	declaring phase = iota
	laid
	ready
)

func (c *Checker) advance(next phase) {
	c.phase = next
	queued := c.waiting[next]
	delete(c.waiting, next)
	for _, run := range queued {
		run()
	}
}

func (c *Checker) when(at phase, run func()) {
	if c.phase >= at {
		run()
		return
	}
	c.waiting[at] = append(c.waiting[at], run)
}

func (c *Checker) created(t *types.Type) {
	switch {
	case t.Kind == types.ResultKind:
		c.when(laid, func() { c.layout(t, diag.Pos{}) })
	case t.Optional:
		if t.Elem.Size() > 0 {
			c.layout(t, diag.Pos{})
		}
	case t.Kind == types.MapKind:
		c.mapOps(t)
	case t.Kind == types.ChanKind:
		c.chanOps(t)
	}
}
