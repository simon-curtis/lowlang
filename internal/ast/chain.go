package ast

func LeftSpine(e *Binary) []*Binary {
	spine := []*Binary{e}
	for {
		next, ok := e.X.(*Binary)
		if !ok {
			return spine
		}
		spine = append(spine, next)
		e = next
	}
}

func ElseChain(e *If) []*If {
	chain := []*If{e}
	for {
		next, ok := e.Else.(*If)
		if !ok {
			return chain
		}
		chain = append(chain, next)
		e = next
	}
}
