package ir

import "slices"

type funcClone struct {
	fn     *Func
	values map[*Value]*Value
	blocks map[*Block]*Block
}

func cloneFunc(f *Func) *funcClone {
	c := &funcClone{
		values: map[*Value]*Value{},
		blocks: map[*Block]*Block{},
	}
	duplicate := *f
	duplicate.Slots = slices.Clone(f.Slots)
	duplicate.Blocks = nil
	c.fn = &duplicate
	for _, b := range f.Blocks {
		c.blocks[b] = &Block{ID: b.ID, Kind: b.Kind, Table: b.Table, Poll: b.Poll}
		duplicate.Blocks = append(duplicate.Blocks, c.blocks[b])
	}
	for _, b := range f.Blocks {
		c.cloneValues(b)
	}
	for _, b := range f.Blocks {
		c.connect(b)
	}
	return c
}

func (c *funcClone) cloneValues(b *Block) {
	target := c.blocks[b]
	for _, v := range b.Values {
		if v.forward != nil {
			continue
		}
		duplicate := *v
		duplicate.Block = target
		duplicate.forward = nil
		c.values[v] = &duplicate
		target.Values = append(target.Values, &duplicate)
	}
}

func (c *funcClone) mapped(v *Value) *Value {
	return c.values[resolve(v)]
}

func (c *funcClone) connect(b *Block) {
	target := c.blocks[b]
	for _, v := range target.Values {
		args := make([]*Value, len(v.Args))
		for i, a := range v.Args {
			args[i] = c.mapped(a)
		}
		v.Args = args
	}
	if b.Control != nil {
		target.Control = c.mapped(b.Control)
	}
	for _, r := range b.Results {
		target.Results = append(target.Results, c.mapped(r))
	}
	for _, p := range b.Preds {
		target.Preds = append(target.Preds, c.blocks[p])
	}
	for _, s := range b.Succs {
		target.Succs = append(target.Succs, c.blocks[s])
	}
}
