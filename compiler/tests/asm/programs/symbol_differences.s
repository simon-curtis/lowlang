
	.intel_syntax noprefix
	.section .rodata
	.p2align 2
.La:
	.long 1, 2, 3
.Lb:
	.long .Lb - .La, .Lend - .La, .Lc - .Lb + 4, .Lc - .La - 4
.Lc:
	.quad .La, .Lc + 16, ext + 8, .Lend
	.long ext2
.Lend:
	.text
.Lt1:
	nop
.Lt2:
	.section .data
	.long .Lt2 - .Lt1
