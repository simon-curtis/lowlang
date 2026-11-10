	.set TLS_M, -24
	.set TLS_G, -16
	.set TLS_FLAG, -8

	.set M_STATE, 0
	.set M_CURG, 8
	.set M_G0TOP, 16
	.set M_G0SP, 24
	.set M_LOCKS, 32
	.set M_PARK, 40
	.set M_QLOCK, 48
	.set M_QHEAD, 56
	.set M_QTAIL, 64
	.set M_TICK, 72
	.set M_LASTTICK, 80
	.set M_NEXT, 88
	.set M_FLAG, 96
	.set M_OSBASE, 104
	.set M_OSLIMIT, 112
	.set M_OSDEALLOC, 120
	.set M_RUNNEXT, 128
	.set M_SPINNING, 136
	.set M_ALLOC_CUR, 144
	.set M_ALLOC_END, 152
	.set M_ALLOC_LO, 160
	.set M_ALLOC_HI, 168
	.set ALLOC_SMALL, 255
	.set ALLOC_BUFFER, 4096
	.set M_QUEUE, 256
	.set M_TLS, 2304
	.set M_TCB, 2328
	.set M_SIZE, 4096

	.set RUNNING, 1
	.set IDLE, 2
	.set INC, 3
	.set STOPPED, 4
	.set HELD, 8

	.set G_STATUS, 0
	.set G_SP, 8
	.set G_SCANLO, 16
	.set G_USERSP, 24
	.set G_PC, 32
	.set G_FP, 40
	.set G_HI, 48
	.set G_LO, 56
	.set G_LOCK, 64
	.set G_TOKEN, 72
	.set G_NEXT, 80
	.set G_LOCKEDM, 88
	.set G_CTX, 104
	.set G_LIMIT, 96

	.set G_FREE, 0
	.set G_RUNNABLE, 1
	.set G_RUNNING, 2
	.set G_WAITING, 3

	.set GUARD, 4096
	.set STACK, 1048576
	.set G_MAP, 1056768
	.set G0_STACK, 8388608
	.set MAXM, 256
	.set MAXG, 1048576
	.set MAXT, 65536
	.set T_WORDS, 5

	.macro SAVE_ALL
	push rax
	push rbx
	push rcx
	push rdx
	push rsi
	push rdi
	push rbp
	push r8
	push r9
	push r10
	push r11
	push r12
	push r13
	push r14
	push r15
	.endm

	.macro RESTORE_ALL
	pop r15
	pop r14
	pop r13
	pop r12
	pop r11
	pop r10
	pop r9
	pop r8
	pop rbp
	pop rdi
	pop rsi
	pop rdx
	pop rcx
	pop rbx
	pop rax
	.endm

	.macro ENTER_RT
	TLS_GET rax, TLS_G
	mov r11, [rsp]
	mov [rax+G_PC], r11
	lea r11, [rsp+8]
	mov [rax+G_USERSP], r11
	mov [rax+G_FP], rbp
	.endm

