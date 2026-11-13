
	.intel_syntax noprefix
	.section .rodata
	.ascii "a\n\t\\\"\101\x41\r\b\f", "second"
	.asciz "zero # ; terminated"
	.p2align 3
	.double 1.5, -2.25e10, 3.0e-5, 0.1, 123456789.123456789, 1e308, 4.9e-324, 0.0, -0.0
	.space 5, 0xAA
	.zero 3
	.p2align 4
	.zero 2 * 8
	.data
	.p2align 2
	.byte 1
	.p2align 3
	.quad 9
	.bss
	.p2align 5
bss_a:
	.zero 10
	.p2align 3
bss_b:
	.zero 4
	.section .tbss,"awT",@nobits
	.p2align 3
tls_a:
	.zero 24
	.section .rodata
	.globl exported
exported:
	.quad exported, bss_b, tls_a
