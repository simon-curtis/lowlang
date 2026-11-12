package builtin

type ID int

const (
	None ID = iota
	Print
	New
	Push
	Pop
	Truncate
	Delete
	Bytes
	Str
	Cstr
	View
	Load
	Store
	Pin
	Unpin
	Embed
	Sqrt
	Floor
	Exp
	Close
	Sleep
	WaitReadable
	WaitWritable
	StrRaw
	SlicePtr
	Bits
	StrView
	Extend
	ExtendBy
	Hash
	Zero
	Syscall
	MulHi
	Make
	Memmove
	Ctz
	ClockNanos
	HashSeed
	Argc
	Argv
	Envv
	Render
	RenderInto
	Address
	Serialize
	SerializeInto
	Spawn
	Current
	Park
	Ready
	Lock
	Unlock
	Cas
	Panic
	Context
	SetContext
	HasContext
	VLoad
	VStore
	VSplat
	VMask
	VShuffle
	VSelect
	VMin
	VMax
	VLane
	VWith
	Chan
	Convert
	VectorCast
)

var names = [...]string{
	None:          "",
	Print:         "print",
	New:           "new",
	Push:          "push",
	Pop:           "pop",
	Truncate:      "truncate",
	Delete:        "delete",
	Bytes:         "bytes",
	Str:           "str",
	Cstr:          "cstr",
	View:          "view",
	Load:          "load",
	Store:         "store",
	Pin:           "pin",
	Unpin:         "unpin",
	Embed:         "embed",
	Sqrt:          "sqrt",
	Floor:         "floor",
	Exp:           "exp",
	Close:         "close",
	Sleep:         "sleep",
	WaitReadable:  "wait_readable",
	WaitWritable:  "wait_writable",
	StrRaw:        "__str_raw",
	SlicePtr:      "__slice_ptr",
	Bits:          "__bits",
	StrView:       "__str_view",
	Extend:        "__extend",
	ExtendBy:      "__extend_by",
	Hash:          "__hash",
	Zero:          "__zero",
	Syscall:       "__syscall",
	MulHi:         "__mulhi",
	Make:          "__make",
	Memmove:       "__memmove",
	Ctz:           "__ctz",
	ClockNanos:    "__clock_nanos",
	HashSeed:      "__hash_seed",
	Argc:          "__argc",
	Argv:          "__argv",
	Envv:          "__envv",
	Render:        "__render",
	RenderInto:    "__render_into",
	Address:       "__address",
	Serialize:     "__serialize",
	SerializeInto: "__serialize_into",
	Spawn:         "__spawn",
	Current:       "__current",
	Park:          "__park",
	Ready:         "__ready",
	Lock:          "__lock",
	Unlock:        "__unlock",
	Cas:           "__cas",
	Panic:         "__panic",
	Context:       "__context",
	SetContext:    "__set_context",
	HasContext:    "__has_context",
	VLoad:         "__vload",
	VStore:        "__vstore",
	VSplat:        "__vsplat",
	VMask:         "__vmask",
	VShuffle:      "__vshuffle",
	VSelect:       "__vselect",
	VMin:          "__vmin",
	VMax:          "__vmax",
	VLane:         "__vlane",
	VWith:         "__vwith",
	Chan:          "chan",
	Convert:       "convert",
	VectorCast:    "vector_cast",
}

func (id ID) String() string {
	return names[id]
}
