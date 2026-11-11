package asm

type Reloc uint8

const (
	RelocNone Reloc = iota
	RelocGOTPCREL
	RelocImgRel
	RelocPLT
)

type Value struct {
	Sym   string
	Neg   string
	Add   int64
	Reloc Reloc
}

func (v Value) IsConst() bool {
	return v.Sym == "" && v.Neg == "" && v.Reloc == RelocNone
}

func (v Value) negated() Value {
	return Value{Sym: v.Neg, Neg: v.Sym, Add: -v.Add, Reloc: v.Reloc}
}

func (v Value) plus(o Value) (Value, bool) {
	sum := Value{Add: v.Add + o.Add}
	if v.Reloc != RelocNone && o.Reloc != RelocNone {
		return sum, false
	}
	sum.Reloc = v.Reloc | o.Reloc
	positives := [2]string{v.Sym, o.Sym}
	negatives := [2]string{v.Neg, o.Neg}
	for i, p := range positives {
		if p == "" {
			continue
		}
		for j, n := range negatives {
			if n == p {
				positives[i], negatives[j] = "", ""
				break
			}
		}
	}
	sum.Sym, sum.Neg = positives[0], negatives[0]
	if sum.Sym == "" {
		sum.Sym = positives[1]
	} else if positives[1] != "" {
		return sum, false
	}
	if sum.Neg == "" {
		sum.Neg = negatives[1]
	} else if negatives[1] != "" {
		return sum, false
	}
	return sum, true
}
