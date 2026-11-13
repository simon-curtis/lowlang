
	.intel_syntax noprefix
	.set MODE, 1
	.macro PAIR a, b
.Lpair_\@:
	.quad \a + \b, .Lpair_\@
	.endm
	.macro OUTER x
	PAIR \x, \x
	.endm
	.section .rodata
	PAIR 1, 2
	OUTER 5
.if MODE
	.quad 11
.if MODE - 1
	.quad 12
.else
	.quad 13
.endif
.else
	.quad 14
.if MODE
	.quad 15
.endif
.endif
1:
	.quad 1b, 2f
2:
	.quad 1b, 1f
1:
	.quad 1b
