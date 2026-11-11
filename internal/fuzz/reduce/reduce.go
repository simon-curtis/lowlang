package reduce

import (
	"context"
	"sync"

	"lowlang/internal/fuzz/model"
)

type Predicate func(ctx context.Context, p *model.Program) bool

type Reducer struct {
	Keep     Predicate
	Workers  int
	MaxTests int

	mu    sync.Mutex
	cache map[string]bool
	tests int
}

func (r *Reducer) Tests() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tests
}

func (r *Reducer) Reduce(ctx context.Context, p *model.Program) *model.Program {
	r.cache = map[string]bool{}
	current := p.Clone()
	for progress := true; progress && r.live(ctx); {
		progress = false
		for _, ps := range passes {
			var improved bool
			current, improved = r.runPass(ctx, current, ps)
			progress = progress || improved
		}
	}
	return current
}

func (r *Reducer) live(ctx context.Context) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ctx.Err() == nil && (r.MaxTests <= 0 || r.tests < r.MaxTests)
}

func sites(p *model.Program, ps pass) int {
	w := counting()
	ps(p, w)
	return w.seen
}

func candidate(p *model.Program, ps pass, index int) *model.Program {
	c := p.Clone()
	ps(c, &walker{target: index})
	return c
}

func (r *Reducer) runPass(ctx context.Context, current *model.Program, ps pass) (*model.Program, bool) {
	improved := false
	count := sites(current, ps)
	for i := 0; i < count && r.live(ctx); {
		batch := min(max(r.Workers, 1), count-i)
		cands := make([]*model.Program, batch)
		for j := range batch {
			cands[j] = candidate(current, ps, i+j)
		}
		kept := r.test(ctx, cands)
		first := -1
		for j, ok := range kept {
			if ok {
				first = j
				break
			}
		}
		if first < 0 {
			i += batch
			continue
		}
		current, improved = cands[first], true
		count = sites(current, ps)
		i += first
	}
	return current, improved
}

func (r *Reducer) test(ctx context.Context, cands []*model.Program) []bool {
	results := make([]bool, len(cands))
	var wg sync.WaitGroup
	for i, c := range cands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = r.accept(ctx, c)
		}()
	}
	wg.Wait()
	return results
}

func (r *Reducer) accept(ctx context.Context, c *model.Program) bool {
	key := c.String()
	r.mu.Lock()
	if known, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return known
	}
	if r.MaxTests > 0 && r.tests >= r.MaxTests {
		r.mu.Unlock()
		return false
	}
	r.tests++
	r.mu.Unlock()
	kept := r.Keep(ctx, c)
	if ctx.Err() != nil {
		return false
	}
	r.mu.Lock()
	r.cache[key] = kept
	r.mu.Unlock()
	return kept
}
