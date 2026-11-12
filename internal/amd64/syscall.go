package amd64

var cheapSyscalls = map[int64]bool{
	3: true, 8: true, 9: true, 10: true, 11: true, 12: true, 13: true, 14: true,
	22: true, 24: true, 27: true, 28: true, 32: true, 33: true, 39: true,
	41: true, 48: true, 49: true, 50: true, 51: true, 52: true, 53: true, 54: true, 55: true,
	56: true, 57: true, 58: true, 59: true, 60: true, 62: true, 63: true,
	79: true, 80: true, 81: true, 95: true, 96: true, 97: true, 98: true,
	102: true, 104: true, 107: true, 108: true, 109: true, 110: true, 111: true, 112: true, 121: true, 124: true,
	131: true, 157: true, 158: true, 186: true, 201: true, 203: true, 204: true, 213: true,
	228: true, 229: true, 231: true, 233: true, 234: true, 273: true, 284: true, 290: true, 291: true, 292: true,
	293: true, 302: true, 318: true,
}

func (in *instr) inlineSyscall() bool {
	number := in.args[0].konst
	return number != nil && cheapSyscalls[*number]
}
