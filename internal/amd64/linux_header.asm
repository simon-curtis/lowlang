	.macro TLS_GET reg, slot
	mov \reg, qword ptr fs:[\slot]
	.endm

	.macro TLS_SET slot, reg
	mov qword ptr fs:[\slot], \reg
	.endm

	.macro TLS_CLEAR_FLAG
	mov byte ptr fs:[TLS_FLAG], 0
	.endm

	.macro TLS_POLL
	cmp byte ptr fs:[TLS_FLAG], 0
	.endm

	.macro TEB_ENTER_OS g, m
	.endm

	.macro TEB_ENTER_G g, m
	.endm

	.macro TEB_ENTER_CURRENT_G m
	.endm

	.macro HEAP_COMMIT reg
	.endm
