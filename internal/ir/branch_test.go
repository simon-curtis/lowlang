package ir

import "testing"

func branchOnConstant(t *testing.T, condition int64) (*Block, *Value) {
	t.Helper()
	fn := &Func{}
	entry := fn.newBlock()
	join := fn.newBlock()
	control := fn.newValue(entry, OpConst)
	control.Aux = condition
	first := fn.newValue(entry, OpConst)
	first.Aux = 1
	second := fn.newValue(entry, OpConst)
	second.Aux = 2
	phi := fn.newValue(join, OpPhi, first, second)
	entry.Values = []*Value{control, first, second}
	entry.Kind = If
	entry.Control = control
	entry.Succs = []*Block{join, join}
	join.Preds = []*Block{entry, entry}
	join.Values = []*Value{phi}
	return entry, phi
}

func TestTakeBranchWithSharedSuccessorKeepsTakenPhiArgument(t *testing.T) {
	for _, tc := range []struct {
		condition int64
		want      int64
	}{{1, 1}, {0, 2}} {
		entry, phi := branchOnConstant(t, tc.condition)
		takeCase(entry, branchIndex(tc.condition != 0))
		if len(phi.Args) != 1 || phi.Args[0].Aux != tc.want {
			t.Errorf("condition %d: phi arguments %v, want the constant %d", tc.condition, phi.Args, tc.want)
		}
		join := entry.Succs[0]
		if len(join.Preds) != 1 || join.Preds[0] != entry {
			t.Errorf("condition %d: join predecessors %v, want one edge from the entry", tc.condition, join.Preds)
		}
	}
}
