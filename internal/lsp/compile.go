package lsp

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"lowlang/internal/compiler"
)

const debounce = 150 * time.Millisecond

type sources map[string][]byte

func (src sources) read(name string) ([]byte, error) {
	if text, ok := src[filepath.Clean(name)]; ok {
		return text, nil
	}
	return os.ReadFile(name)
}

type compileJob struct {
	entry   string
	path    string
	version int
	docs    sources
}

type compiled struct {
	job  compileJob
	unit *compiler.Unit
	err  error
}

func (j compileJob) run() (result compiled) {
	result.job = j
	defer func() {
		if r := recover(); r != nil {
			result.err = fmt.Errorf("internal compiler error: %v", r)
		}
	}()
	result.unit, result.err = compiler.Check(j.entry, j.docs.read)
	return result
}

type scheduler struct {
	versions  map[string]int
	dirty     map[string]string
	compiling bool
	timer     *time.Timer
	results   chan compiled
}

func newScheduler() *scheduler {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	return &scheduler{
		versions: map[string]int{},
		dirty:    map[string]string{},
		timer:    timer,
		results:  make(chan compiled),
	}
}

func (c *scheduler) mark(entry, path string, delay time.Duration) {
	c.versions[entry]++
	c.dirty[entry] = path
	c.timer.Reset(delay)
}

func (c *scheduler) next(ctx context.Context, docs sources) {
	if c.compiling {
		return
	}
	for entry, path := range c.dirty {
		delete(c.dirty, entry)
		c.compiling = true
		job := compileJob{entry: entry, path: path, version: c.versions[entry], docs: maps.Clone(docs)}
		go c.compile(ctx, job)
		return
	}
}

func (c *scheduler) compile(ctx context.Context, job compileJob) {
	result := job.run()
	select {
	case c.results <- result:
	case <-ctx.Done():
	}
}

func (c *scheduler) finish(r compiled) bool {
	c.compiling = false
	if len(c.dirty) > 0 {
		c.timer.Reset(debounce)
	}
	return r.job.version == c.versions[r.job.entry]
}
