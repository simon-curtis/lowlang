package ir

type Dominators[N comparable] struct {
	Nodes    []N
	Index    map[N]int
	Idom     []int
	Children [][]int
	preds    func(N) []N
}

func NewDominators[N comparable](entry N, succs, preds func(N) []N) *Dominators[N] {
	d := &Dominators[N]{Index: map[N]int{}, preds: preds}
	d.order(entry, succs)
	n := len(d.Nodes)
	d.Idom = make([]int, n)
	for i := range d.Idom {
		d.Idom[i] = -1
	}
	d.Idom[0] = 0
	for changed := true; changed; {
		changed = false
		for i := 1; i < n; i++ {
			if d.refine(i) {
				changed = true
			}
		}
	}
	d.Children = make([][]int, n)
	for i := 1; i < n; i++ {
		if d.Idom[i] >= 0 {
			d.Children[d.Idom[i]] = append(d.Children[d.Idom[i]], i)
		}
	}
	return d
}

func (d *Dominators[N]) order(entry N, succs func(N) []N) {
	seen := map[N]bool{}
	var post []N
	var visit func(node N)
	visit = func(node N) {
		seen[node] = true
		for _, s := range succs(node) {
			if !seen[s] {
				visit(s)
			}
		}
		post = append(post, node)
	}
	visit(entry)
	for i := len(post) - 1; i >= 0; i-- {
		d.Index[post[i]] = len(d.Nodes)
		d.Nodes = append(d.Nodes, post[i])
	}
}

func (d *Dominators[N]) refine(i int) bool {
	next := -1
	for _, p := range d.preds(d.Nodes[i]) {
		pi, reachable := d.Index[p]
		if !reachable || d.Idom[pi] < 0 {
			continue
		}
		if next < 0 {
			next = pi
			continue
		}
		next = d.Intersect(pi, next)
	}
	if next == d.Idom[i] {
		return false
	}
	d.Idom[i] = next
	return true
}

func (d *Dominators[N]) Intersect(a, b int) int {
	for a != b {
		for a > b {
			a = d.Idom[a]
		}
		for b > a {
			b = d.Idom[b]
		}
	}
	return a
}
