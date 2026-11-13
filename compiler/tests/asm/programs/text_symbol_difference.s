
	.intel_syntax noprefix
	.text
	.globl entry
entry:
.Lt1:
	nop
	jmp .Lt3
	mov rax, 5
.Lt2:
	.p2align 4
.Lt3:
	ret
	.section .rodata
	.long 7, .Lt2 - .Lt1, .Lt3 - .Lt1
