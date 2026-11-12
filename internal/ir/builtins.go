package ir

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/prelude"
	"lowlang/internal/types"
)

type lowering func(b *builder, e *ast.Call) []*Value

var lowerings map[builtin.ID]lowering

func init() {
	lowerings = map[builtin.ID]lowering{
		builtin.Print:        (*builder).printCall,
		builtin.New:          (*builder).newCall,
		builtin.Push:         (*builder).pushCall,
		builtin.Pop:          (*builder).pop,
		builtin.Truncate:     (*builder).truncateCall,
		builtin.Delete:       (*builder).deleteCall,
		builtin.Bytes:        (*builder).bytesCall,
		builtin.Str:          (*builder).strCall,
		builtin.Cstr:         (*builder).cstrCall,
		builtin.View:         (*builder).viewCall,
		builtin.Load:         (*builder).loadCall,
		builtin.Store:        (*builder).storeCall,
		builtin.Pin:          (*builder).pinCall,
		builtin.Unpin:        (*builder).unpinCall,
		builtin.Embed:        (*builder).embedCall,
		builtin.Sqrt:         (*builder).sqrtCall,
		builtin.Floor:        (*builder).floorCall,
		builtin.Exp:          (*builder).expCall,
		builtin.Close:        (*builder).closeCall,
		builtin.Sleep:        (*builder).sleepCall,
		builtin.WaitReadable: (*builder).waitReadableCall,
		builtin.WaitWritable: (*builder).waitWritableCall,
		builtin.StrRaw:       (*builder).strRawCall,
		builtin.SlicePtr:     (*builder).slicePtrCall,
		builtin.Bits:         (*builder).bitsCall,
		builtin.StrView:      (*builder).strViewCall,
		builtin.Extend:       (*builder).extendCall,
		builtin.ExtendBy:     (*builder).extendByCall,
		builtin.Hash:         (*builder).hashCall,
		builtin.Zero:         (*builder).zeroCall,
		builtin.Syscall:      (*builder).syscallCall,
		builtin.MulHi:        (*builder).mulhiCall,
		builtin.Make:         (*builder).makeCall,
		builtin.Memmove:      (*builder).memmoveCall,
		builtin.Ctz:          (*builder).ctzCall,
		builtin.ClockNanos:   (*builder).clockNanosCall,
		builtin.HashSeed:     (*builder).hashSeedCall,
		builtin.Argc:         (*builder).argcCall,
		builtin.Argv:         (*builder).argvCall,
		builtin.Envv:         (*builder).envvCall,
		builtin.Render:       (*builder).renderCall,
		builtin.Address:      (*builder).addressCall,
		builtin.Serialize:    (*builder).serializeCall,
		builtin.Spawn:        (*builder).spawnCall,
		builtin.Current:      (*builder).currentCall,
		builtin.Park:         (*builder).parkCall,
		builtin.Ready:        (*builder).readyCall,
		builtin.Lock:         (*builder).lockCall,
		builtin.Unlock:       (*builder).unlockCall,
		builtin.Cas:          (*builder).casCall,
		builtin.Panic:        (*builder).panicCall,
		builtin.Context:      (*builder).contextCall,
		builtin.SetContext:   (*builder).setContextCall,
		builtin.HasContext:   (*builder).hasContextCall,
		builtin.VLoad:        (*builder).vectorValue,
		builtin.VStore:       (*builder).vstoreCall,
		builtin.VSplat:       (*builder).vectorValue,
		builtin.VMask:        (*builder).vmaskCall,
		builtin.VShuffle:     (*builder).vectorValue,
		builtin.VSelect:      (*builder).vectorValue,
		builtin.VMin:         (*builder).vectorValue,
		builtin.VMax:         (*builder).vectorValue,
		builtin.VLane:        (*builder).vlaneCall,
		builtin.VWith:        (*builder).vectorValue,
		builtin.Chan:         (*builder).chanCall,
		builtin.Convert:      (*builder).convertCall,
		builtin.VectorCast:   (*builder).vectorValue,
	}
}

func (b *builder) builtin(e *ast.Call) []*Value {
	lower, ok := lowerings[e.Builtin]
	if !ok {
		panic(fmt.Sprintf("ir: no lowering for builtin %s", e.Builtin))
	}
	return lower(b, e)
}

func (b *builder) newCall(e *ast.Call) []*Value {
	ptr := b.allocate(e.Args[0])
	return []*Value{ptr}
}

func (b *builder) convertCall(e *ast.Call) []*Value {
	from := e.Args[0].TypeOf()
	word := b.word(e.Args[0])
	return []*Value{b.convert(word, from, e.Type)}
}

func (b *builder) sqrtCall(e *ast.Call) []*Value {
	word := b.word(e.Args[0])
	return []*Value{b.typed(OpSqrt, e.Type, word)}
}

func (b *builder) floorCall(e *ast.Call) []*Value {
	word := b.word(e.Args[0])
	return []*Value{b.typed(OpFloor, e.Type, word)}
}

func (b *builder) expCall(e *ast.Call) []*Value {
	word := b.word(e.Args[0])
	return []*Value{b.exp(word, e.Type)}
}

func (b *builder) pushCall(e *ast.Call) []*Value {
	b.push(e)
	return nil
}

func (b *builder) truncateCall(e *ast.Call) []*Value {
	b.truncate(e)
	return nil
}

func (b *builder) deleteCall(e *ast.Call) []*Value {
	b.deleteEntry(e)
	return nil
}

func (b *builder) extendCall(e *ast.Call) []*Value {
	b.extend(e)
	return nil
}

func (b *builder) extendByCall(e *ast.Call) []*Value {
	length := b.extendBy(e)
	return []*Value{length}
}

func (b *builder) embedCall(e *ast.Call) []*Value {
	v := b.emit(OpEmbed)
	v.Str = e.Embed
	v.Aux = int64(e.Digest)
	v.Scalar = true
	return []*Value{v, b.constant(e.Size)}
}

func (b *builder) bytesCall(e *ast.Call) []*Value {
	text := b.value(e.Args[0], types.Str)
	return b.callNamed(prelude.Bytes, text...)
}

func (b *builder) strCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	if arg.TypeOf().Kind == types.RawKind {
		return b.callNamed(prelude.FromCstr, b.word(arg))
	}
	bytes := b.value(arg, arg.TypeOf())
	return b.callNamed(prelude.FromBytes, bytes...)
}

func (b *builder) cstrCall(e *ast.Call) []*Value {
	text := b.value(e.Args[0], types.Str)
	return b.callNamed(prelude.Cstr, text...)
}

func (b *builder) viewCall(e *ast.Call) []*Value {
	ptr := b.word(e.Args[0])
	n := b.word(e.Args[1])
	return []*Value{ptr, n, n}
}

func (b *builder) loadCall(e *ast.Call) []*Value {
	elem := e.Type
	ptr := b.word(e.Args[0])
	return b.load(place{addr: ptr, scalar: elem.Width(), tail: elem.Tail(), mem: MemRaw | MemTrap}, elem)
}

func (b *builder) storeCall(e *ast.Call) []*Value {
	elem := e.Args[1].TypeOf()
	ptr := b.word(e.Args[0])
	value := b.value(e.Args[1], elem)
	b.store(place{addr: ptr, scalar: elem.Width(), tail: elem.Tail(), mem: MemRaw}, value)
	return nil
}

func (b *builder) pinCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	word := b.value(arg, arg.TypeOf())[0]
	b.runtime(rtPin, word)
	return []*Value{word}
}

func (b *builder) unpinCall(e *ast.Call) []*Value {
	ptr := b.word(e.Args[0])
	b.runtime(rtUnpin, ptr)
	return nil
}

func (b *builder) strRawCall(e *ast.Call) []*Value {
	ptr := b.word(e.Args[0])
	n := b.word(e.Args[1])
	return []*Value{ptr, n}
}

func (b *builder) slicePtrCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	return b.value(arg, arg.TypeOf())[:1]
}

func (b *builder) bitsCall(e *ast.Call) []*Value {
	return []*Value{b.word(e.Args[0])}
}

func (b *builder) addressCall(e *ast.Call) []*Value {
	return []*Value{b.word(e.Args[0])}
}

func (b *builder) strViewCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	return b.value(arg, arg.TypeOf())[:2]
}

func (b *builder) hashCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	if arg.TypeOf() == types.Str {
		text := b.value(arg, types.Str)
		return b.callNamed(prelude.HashStr, text...)
	}
	return b.callNamed(prelude.HashWord, b.word(arg))
}

func (b *builder) zeroCall(e *ast.Call) []*Value {
	zero := b.constant(0)
	words := make([]*Value, e.Type.Size())
	for k := range words {
		words[k] = zero
	}
	return words
}

func (b *builder) syscallCall(e *ast.Call) []*Value {
	words := make([]*Value, len(e.Args))
	for i, arg := range e.Args {
		words[i] = b.word(arg)
	}
	return []*Value{b.syscall(words)}
}

func (b *builder) mulhiCall(e *ast.Call) []*Value {
	x := b.word(e.Args[0])
	y := b.word(e.Args[1])
	return []*Value{b.typed(OpMulHigh, types.U64, x, y)}
}

func (b *builder) makeCall(e *ast.Call) []*Value {
	n := b.word(e.Args[0])
	return b.make(e.Type.Elem, n)
}

func (b *builder) memmoveCall(e *ast.Call) []*Value {
	n := b.copySlice(e)
	return []*Value{n}
}

func (b *builder) ctzCall(e *ast.Call) []*Value {
	word := b.word(e.Args[0])
	return []*Value{b.typed(OpTrailingZeros, types.I64, word)}
}

func (b *builder) clockNanosCall(e *ast.Call) []*Value {
	clock := b.word(e.Args[0])
	return []*Value{b.runtime(rtClockNanos, clock)}
}

func (b *builder) hashSeedCall(e *ast.Call) []*Value {
	seed := b.typed(OpHashSeed, types.U64)
	seed.Aux = e.Args[0].(*ast.IntLit).Value
	return []*Value{seed}
}

func (b *builder) argcCall(e *ast.Call) []*Value {
	return []*Value{b.runtime(rtArgc)}
}

func (b *builder) argvCall(e *ast.Call) []*Value {
	index := b.word(e.Args[0])
	return []*Value{b.runtime(rtArgv, index)}
}

func (b *builder) envvCall(e *ast.Call) []*Value {
	index := b.word(e.Args[0])
	return []*Value{b.runtime(rtEnvv, index)}
}

func (b *builder) renderCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	words := b.value(arg, arg.TypeOf())
	return b.renderString(e.Render, words)
}

func (b *builder) serializeCall(e *ast.Call) []*Value {
	b.serialize(e)
	return nil
}

func (b *builder) printCall(e *ast.Call) []*Value {
	arg := e.Args[0]
	var text []*Value
	if e.Render == nil {
		text = b.value(arg, types.Str)
	} else {
		words := b.value(arg, arg.TypeOf())
		text = b.renderString(e.Render, words)
	}
	b.runtime(rtPrintStr, text...)
	return nil
}

func (b *builder) chanCall(e *ast.Call) []*Value {
	capacity := b.constant(0)
	if len(e.Args) == 1 {
		capacity = b.word(e.Args[0])
	}
	return b.chanNamed(prelude.ChanNew, e.Type, capacity)
}

func (b *builder) closeCall(e *ast.Call) []*Value {
	c := b.word(e.Args[0])
	b.chanNamed(prelude.ChanClose, e.Args[0].TypeOf(), c)
	return nil
}

func (b *builder) sleepCall(e *ast.Call) []*Value {
	b.runtime(rtSleep, b.word(e.Args[0]))
	return nil
}

func (b *builder) waitReadableCall(e *ast.Call) []*Value {
	b.runtime(rtWaitFd, b.word(e.Args[0]), b.constant(1))
	return nil
}

func (b *builder) waitWritableCall(e *ast.Call) []*Value {
	b.runtime(rtWaitFd, b.word(e.Args[0]), b.constant(4))
	return nil
}

func (b *builder) spawnCall(e *ast.Call) []*Value {
	f := b.value(e.Args[0], e.Args[0].TypeOf())
	b.runtime(rtSpawn, f...)
	return nil
}

func (b *builder) currentCall(e *ast.Call) []*Value {
	return []*Value{b.runtime(rtCurrent)}
}

func (b *builder) parkCall(e *ast.Call) []*Value {
	b.runtime(rtPark)
	return nil
}

func (b *builder) contextCall(e *ast.Call) []*Value {
	return []*Value{b.runtime(rtContext)}
}

func (b *builder) setContextCall(e *ast.Call) []*Value {
	b.runtime(rtSetContext, b.word(e.Args[0]))
	return nil
}

func (b *builder) hasContextCall(e *ast.Call) []*Value {
	return []*Value{b.runtime(rtHasContext)}
}

func (b *builder) readyCall(e *ast.Call) []*Value {
	b.runtime(rtReady, b.word(e.Args[0]))
	return nil
}

func (b *builder) lockCall(e *ast.Call) []*Value {
	b.runtime(rtLock, b.refAddress(e.Args[0]))
	return nil
}

func (b *builder) unlockCall(e *ast.Call) []*Value {
	b.runtime(rtUnlock, b.refAddress(e.Args[0]))
	return nil
}

func (b *builder) casCall(e *ast.Call) []*Value {
	addr := b.refAddress(e.Args[0])
	old := b.word(e.Args[1])
	next := b.word(e.Args[2])
	return []*Value{b.runtime(rtCas, addr, old, next)}
}

func (b *builder) panicCall(e *ast.Call) []*Value {
	msg := b.value(e.Args[0], types.Str)
	b.runtime(rtPanicMsg, msg...)
	b.cur.Kind = Unreachable
	b.dead()
	return nil
}

func (b *builder) vectorValue(e *ast.Call) []*Value {
	addr := b.vectorCall(e)
	return b.load(place{addr: addr}, e.Type)
}

func (b *builder) vstoreCall(e *ast.Call) []*Value {
	t := e.Args[2].TypeOf()
	dst := b.vectorAddress(e.Args[0], e.Args[1], t)
	b.vectorOp(VCopy, t, dst, b.vec(e.Args[2]))
	return nil
}

func (b *builder) vmaskCall(e *ast.Call) []*Value {
	t := e.Args[0].TypeOf()
	v := b.typed(OpVectorScalar, types.U64, b.vec(e.Args[0]))
	v.Aux = int64(VMask)
	v.From = t
	return []*Value{v}
}

func (b *builder) vlaneCall(e *ast.Call) []*Value {
	t := e.Args[0].TypeOf()
	return b.load(b.lanePlace(b.vec(e.Args[0]), e.Args[1], t), t.Elem)
}
