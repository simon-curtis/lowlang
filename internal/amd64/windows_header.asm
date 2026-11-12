	.set TEB_STACK_BASE, 0x8
	.set TEB_STACK_LIMIT, 0x10
	.set TEB_DEALLOCATION_STACK, 0x1478
	.set TEB_TLS_SLOTS, 0x1480
	.set TLS_SLOT_COUNT, 64
	.set STACK_COMMIT, 16384
	.set HEAP_CHUNK, 1048576
	.set MEM_COMMIT, 0x1000
	.set MEM_RESERVE, 0x2000
	.set PAGE_READWRITE, 4
	.set PAGE_GUARDED, 0x104
	.set STACK_RESERVATION, 0x10000
	.set WT_EXECUTEONLYONCE, 8
	.set EXCEPTION_ACCESS_VIOLATION, 0xC0000005
	.set EXCEPTION_STACK_OVERFLOW, 0xC00000FD
	.set EXCEPTION_ADDRESS, 0x28
	.set CONTEXT_RSP, 0x98
	.set CONTEXT_RDI, 0xB0
	.set CONTEXT_RIP, 0xF8

	.macro TLS_GET reg, slot
	mov \reg, qword ptr [rip+rt_tls_slot]
	mov \reg, qword ptr gs:[\reg]
	mov \reg, qword ptr [\reg+\slot]
	.endm

	.macro TLS_SET slot, reg
	mov r11, qword ptr [rip+rt_tls_slot]
	mov r11, qword ptr gs:[r11]
	mov qword ptr [r11+\slot], \reg
	.endm

	.macro TLS_CLEAR_FLAG
	mov r11, qword ptr [rip+rt_tls_slot]
	mov r11, qword ptr gs:[r11]
	mov byte ptr [r11+TLS_FLAG], 0
	.endm

	.macro TLS_POLL
	mov r11, qword ptr [rip+rt_tls_slot]
	mov r11, qword ptr gs:[r11]
	cmp byte ptr [r11+TLS_FLAG], 0
	.endm

	.macro TEB_OS m
	mov r11, [\m+M_OSBASE]
	mov qword ptr gs:[TEB_STACK_BASE], r11
	mov r11, [\m+M_OSLIMIT]
	mov qword ptr gs:[TEB_STACK_LIMIT], r11
	mov r11, [\m+M_OSDEALLOC]
	mov qword ptr gs:[TEB_DEALLOCATION_STACK], r11
	.endm

	.macro TEB_ENTER_OS g, m
	mov r11, qword ptr gs:[TEB_STACK_LIMIT]
	mov [\g+G_LIMIT], r11
	TEB_OS \m
	.endm

	.macro TEB_ENTER_G g, m
	mov r11, qword ptr gs:[TEB_STACK_LIMIT]
	mov [\m+M_OSLIMIT], r11
	mov r11, [\g+G_HI]
	mov qword ptr gs:[TEB_STACK_BASE], r11
	mov r11, [\g+G_LIMIT]
	mov qword ptr gs:[TEB_STACK_LIMIT], r11
	mov r11, [\g+G_LO]
	mov qword ptr gs:[TEB_DEALLOCATION_STACK], r11
	.endm

	.macro TEB_ENTER_CURRENT_G m
	mov r10, [\m+M_CURG]
	TEB_ENTER_G r10, \m
	.endm

	.macro HEAP_COMMIT reg
	cmp \reg, [rip+rt_heap_committed]
	jbe .Lheap_committed\@
	push rax
	push rcx
	push rdx
	push rsi
	push rdi
	push r8
	push r9
	push r10
	push r11
	mov rdi, \reg
	call rt_heap_commit
	pop r11
	pop r10
	pop r9
	pop r8
	pop rdi
	pop rsi
	pop rdx
	pop rcx
	pop rax
.Lheap_committed\@:
	.endm

	.macro OS_ENTER
	push rbp
	mov rbp, rsp
	push rbx
	push rdx
	push r8
	push r9
	push r10
	and rsp, -16
	sub rsp, 96
	.endm

	.macro OS_LEAVE
	lea rsp, [rbp-40]
	pop r10
	pop r9
	pop r8
	pop rdx
	pop rbx
	pop rbp
	.endm

	.macro WIN fn
	call qword ptr [rip+__imp_\fn]
	.endm
