package regex

const maxProgram = 100000

type Op int

const (
	Char Op = iota
	Class
	Split
	Jump
	Save
	Check
	Done
)

type Inst struct {
	Op     Op
	C      uint32
	Ranges []Range
	X, Y   int
	Look   Look
}

type Program struct {
	Insts  []Inst
	Groups int
}

func Compile(pattern string) (*Program, error) {
	root, groups, err := parse(pattern)
	if err != nil {
		return nil, err
	}
	a := &assembler{code: []Inst{{Op: Save, X: 0}}}
	if err := a.node(root); err != nil {
		return nil, err
	}
	if _, err := a.emit(Inst{Op: Save, X: 1}); err != nil {
		return nil, err
	}
	if _, err := a.emit(Inst{Op: Done}); err != nil {
		return nil, err
	}
	return &Program{Insts: a.code, Groups: groups}, nil
}

type assembler struct {
	code []Inst
}

func (a *assembler) emit(inst Inst) (int, error) {
	if len(a.code) >= maxProgram {
		return 0, invalid(0, "pattern too large")
	}
	a.code = append(a.code, inst)
	return len(a.code) - 1, nil
}

func (a *assembler) node(n node) error {
	switch n := n.(type) {
	case literal:
		_, err := a.emit(Inst{Op: Char, C: n.c})
		return err
	case set:
		_, err := a.emit(Inst{Op: Class, Ranges: n.ranges})
		return err
	case assert:
		_, err := a.emit(Inst{Op: Check, Look: n.look})
		return err
	case capture:
		if _, err := a.emit(Inst{Op: Save, X: 2 * n.index}); err != nil {
			return err
		}
		if err := a.node(n.body); err != nil {
			return err
		}
		_, err := a.emit(Inst{Op: Save, X: 2*n.index + 1})
		return err
	case concat:
		for _, item := range n.items {
			if err := a.node(item); err != nil {
				return err
			}
		}
	case alternate:
		return a.alternate(n.branches)
	case repeat:
		if n.max < 0 {
			return a.unbounded(n.body, n.min, n.greedy)
		}
		return a.bounded(n.body, n.min, n.max, n.greedy)
	}
	return nil
}

func (a *assembler) alternate(branches []node) error {
	var exits []int
	for i, branch := range branches {
		if i == len(branches)-1 {
			if err := a.node(branch); err != nil {
				return err
			}
			continue
		}
		split, err := a.emit(Inst{Op: Split})
		if err != nil {
			return err
		}
		if err := a.node(branch); err != nil {
			return err
		}
		exit, err := a.emit(Inst{Op: Jump})
		if err != nil {
			return err
		}
		exits = append(exits, exit)
		a.code[split] = Inst{Op: Split, X: split + 1, Y: len(a.code)}
	}
	for _, exit := range exits {
		a.code[exit] = Inst{Op: Jump, X: len(a.code)}
	}
	return nil
}

func (a *assembler) bounded(body node, low, high int, greedy bool) error {
	for range low {
		if err := a.node(body); err != nil {
			return err
		}
	}
	var splits []int
	for range high - low {
		split, err := a.emit(Inst{Op: Split})
		if err != nil {
			return err
		}
		splits = append(splits, split)
		if err := a.node(body); err != nil {
			return err
		}
	}
	for _, split := range splits {
		a.code[split] = branch(split+1, len(a.code), greedy)
	}
	return nil
}

func (a *assembler) unbounded(body node, low int, greedy bool) error {
	for i := 1; i < low; i++ {
		if err := a.node(body); err != nil {
			return err
		}
	}
	skip := -1
	if low == 0 {
		var err error
		if skip, err = a.emit(Inst{Op: Split}); err != nil {
			return err
		}
	}
	loop := len(a.code)
	if err := a.node(body); err != nil {
		return err
	}
	back, err := a.emit(Inst{Op: Split})
	if err != nil {
		return err
	}
	a.code[back] = branch(loop, len(a.code), greedy)
	if skip >= 0 {
		a.code[skip] = branch(loop, len(a.code), greedy)
	}
	return nil
}

func branch(more, out int, greedy bool) Inst {
	if greedy {
		return Inst{Op: Split, X: more, Y: out}
	}
	return Inst{Op: Split, X: out, Y: more}
}
