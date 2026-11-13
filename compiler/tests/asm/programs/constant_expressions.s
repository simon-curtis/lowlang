
	.intel_syntax noprefix
	.set A, -5
	.set B, (1 << 4) | 3
	.set C, A * 3 + B
	.set D, 0x10 - 0b101 + 010
	.set E, D
	.set D, 7
	.section .rodata
	.p2align 3
v:
	.quad A, B, C, D, E, ~0, 100 / 7, 100 % 7, -A
	.quad 1 + 2 * 3, 2 * 3 + 1, 1 << 2 + 1, 6 | 1 + 1, 8 >> 1 - 1
	.long B >> 2, 7 & 3, 6 ^ 3, 2 | 8
	.short -1, 0xffff, 65535
	.byte 255, -128, 0x7f
	.quad 0xFFFFFFFFFFFFFFFF, -9223372036854775808, 9223372036854775807
	.long 4294967295, -2147483648
