package ir

type Convention uint8

const (
	CallPlain Convention = iota
	CallGrow
	CallStrEq
)

type RuntimeEntry struct {
	Symbol string
	Conv   Convention
	Void   bool
	Scalar bool
}

var runtimeEntries []*RuntimeEntry

func declare(entry RuntimeEntry) *RuntimeEntry {
	runtimeEntries = append(runtimeEntries, &entry)
	return &entry
}

var (
	rtAlloc      = declare(RuntimeEntry{Symbol: "rt_alloc"})
	rtGrow       = declare(RuntimeEntry{Symbol: "rt_grow", Conv: CallGrow})
	rtMemmove    = declare(RuntimeEntry{Symbol: "rt_memmove", Void: true})
	rtStrEq      = declare(RuntimeEntry{Symbol: "rt_str_eq", Conv: CallStrEq, Scalar: true})
	rtExp        = declare(RuntimeEntry{Symbol: "rt_exp", Scalar: true})
	rtPrintStr   = declare(RuntimeEntry{Symbol: "rt_print_str", Void: true})
	rtPanicMsg   = declare(RuntimeEntry{Symbol: "rt_panic_msg", Void: true})
	rtSleep      = declare(RuntimeEntry{Symbol: "rt_sleep", Void: true})
	rtWaitRead   = declare(RuntimeEntry{Symbol: "rt_wait_readable", Void: true})
	rtWaitWrite  = declare(RuntimeEntry{Symbol: "rt_wait_writable", Void: true})
	rtSpawn      = declare(RuntimeEntry{Symbol: "rt_spawn", Void: true})
	rtCurrent    = declare(RuntimeEntry{Symbol: "rt_current", Scalar: true})
	rtPark       = declare(RuntimeEntry{Symbol: "rt_park", Void: true})
	rtReady      = declare(RuntimeEntry{Symbol: "rt_ready", Void: true})
	rtLock       = declare(RuntimeEntry{Symbol: "rt_lock", Void: true})
	rtUnlock     = declare(RuntimeEntry{Symbol: "rt_unlock", Void: true})
	rtCas        = declare(RuntimeEntry{Symbol: "rt_cas", Scalar: true})
	rtContext    = declare(RuntimeEntry{Symbol: "rt_context"})
	rtSetContext = declare(RuntimeEntry{Symbol: "rt_set_context", Void: true})
	rtHasContext = declare(RuntimeEntry{Symbol: "rt_has_context", Scalar: true})
	rtLockAll    = declare(RuntimeEntry{Symbol: "rt_lock_all", Void: true})
	rtUnlockAll  = declare(RuntimeEntry{Symbol: "rt_unlock_all", Void: true})
	rtTimerAdd   = declare(RuntimeEntry{Symbol: "rt_timer_add", Scalar: true})
	rtTimerDel   = declare(RuntimeEntry{Symbol: "rt_timer_del", Void: true})
	rtPin        = declare(RuntimeEntry{Symbol: "rt_pin", Void: true})
	rtUnpin      = declare(RuntimeEntry{Symbol: "rt_unpin", Void: true})
	rtArgc       = declare(RuntimeEntry{Symbol: "rt_argc", Scalar: true})
	rtArgv       = declare(RuntimeEntry{Symbol: "rt_argv", Scalar: true})
	rtEnvv       = declare(RuntimeEntry{Symbol: "rt_envv", Scalar: true})
	rtClockNanos = declare(RuntimeEntry{Symbol: "rt_clock_nanos", Scalar: true})
	rtSysblock   = declare(RuntimeEntry{Symbol: "rt_sysblock", Scalar: true})
)

func RuntimeEntries() []*RuntimeEntry {
	return runtimeEntries
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
