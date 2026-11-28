	.intel_syntax noprefix
	.text
.Lt1:
	nop
.Lt2:
	ret
	.section .rodata
	.p2align 2
.Ltable:
	.long .Lt1 - ., .Lt2 - . + 4, ext - ., .Ltable - .
