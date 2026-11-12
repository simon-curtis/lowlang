package ir

type Convention uint8

const (
	CallPlain Convention = iota
	CallGrow
	CallSyscall
	CallStrEq
)

type RuntimeEntry struct {
	Symbol string
	Conv   Convention
	Void   bool
	Scalar bool
	Alloc  bool
}

var (
	rtAlloc      = &RuntimeEntry{Symbol: "rt_alloc", Alloc: true}
	rtGrow       = &RuntimeEntry{Symbol: "rt_grow", Conv: CallGrow}
	rtMemmove    = &RuntimeEntry{Symbol: "rt_memmove", Void: true}
	rtStrEq      = &RuntimeEntry{Symbol: "rt_str_eq", Conv: CallStrEq, Scalar: true}
	rtExp        = &RuntimeEntry{Symbol: "rt_exp", Scalar: true}
	rtPrintStr   = &RuntimeEntry{Symbol: "rt_print_str", Void: true}
	rtPanicMsg   = &RuntimeEntry{Symbol: "rt_panic_msg", Void: true}
	rtSleep      = &RuntimeEntry{Symbol: "rt_sleep", Void: true}
	rtWaitFd     = &RuntimeEntry{Symbol: "rt_wait_fd", Void: true}
	rtSpawn      = &RuntimeEntry{Symbol: "rt_spawn", Void: true}
	rtCurrent    = &RuntimeEntry{Symbol: "rt_current", Scalar: true}
	rtPark       = &RuntimeEntry{Symbol: "rt_park", Void: true}
	rtReady      = &RuntimeEntry{Symbol: "rt_ready", Void: true}
	rtLock       = &RuntimeEntry{Symbol: "rt_lock", Void: true}
	rtUnlock     = &RuntimeEntry{Symbol: "rt_unlock", Void: true}
	rtCas        = &RuntimeEntry{Symbol: "rt_cas", Scalar: true}
	rtContext    = &RuntimeEntry{Symbol: "rt_context"}
	rtSetContext = &RuntimeEntry{Symbol: "rt_set_context", Void: true}
	rtHasContext = &RuntimeEntry{Symbol: "rt_has_context", Scalar: true}
	rtLockAll    = &RuntimeEntry{Symbol: "rt_lock_all", Void: true}
	rtUnlockAll  = &RuntimeEntry{Symbol: "rt_unlock_all", Void: true}
	rtTimerAdd   = &RuntimeEntry{Symbol: "rt_timer_add", Scalar: true}
	rtTimerDel   = &RuntimeEntry{Symbol: "rt_timer_del", Void: true}
	rtPin        = &RuntimeEntry{Symbol: "rt_pin", Void: true}
	rtUnpin      = &RuntimeEntry{Symbol: "rt_unpin", Void: true}
	rtArgc       = &RuntimeEntry{Symbol: "rt_argc", Scalar: true}
	rtArgv       = &RuntimeEntry{Symbol: "rt_argv", Scalar: true}
	rtEnvv       = &RuntimeEntry{Symbol: "rt_envv", Scalar: true}
	rtClockNanos = &RuntimeEntry{Symbol: "rt_clock_nanos", Scalar: true}
	rtSysblock   = &RuntimeEntry{Symbol: "rt_sysblock", Conv: CallSyscall, Scalar: true}
	rtPrintBool  = &RuntimeEntry{Symbol: "rt_print_bool", Void: true}
	rtPrintI64   = &RuntimeEntry{Symbol: "rt_print_i64", Void: true}
)

func RuntimeEntries() []*RuntimeEntry {
	return []*RuntimeEntry{
		rtAlloc, rtGrow, rtMemmove, rtStrEq, rtExp, rtPrintStr, rtPrintBool, rtPrintI64, rtPanicMsg, rtSleep, rtWaitFd,
		rtSpawn, rtCurrent, rtPark, rtReady, rtLock, rtUnlock, rtCas, rtContext, rtSetContext, rtHasContext,
		rtLockAll, rtUnlockAll, rtTimerAdd, rtTimerDel, rtPin, rtUnpin, rtArgc, rtArgv, rtEnvv, rtClockNanos, rtSysblock,
	}
}

func (b *builder) runtimeCall(entry *RuntimeEntry, args ...*Value) *Value {
	call := b.emit(OpRuntime, args...)
	call.Runtime = entry
	return call
}

func (b *builder) runtime(entry *RuntimeEntry, args ...*Value) *Value {
	call := b.runtimeCall(entry, args...)
	if entry.Void {
		return nil
	}
	result := b.result(call, 0)
	result.Scalar = entry.Scalar
	return result
}

func (b *builder) syscall(words []*Value) *Value {
	call := b.emit(OpSyscall, words...)
	call.Runtime = rtSysblock
	result := b.result(call, 0)
	result.Scalar = true
	return result
}
