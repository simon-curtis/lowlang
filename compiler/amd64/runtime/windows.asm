	.section .rodata
	MSG_BEGIN rt_msg_tls
	.ascii "no thread-local storage slot"
	MSG_END

	.bss
	.p2align 3
rt_tls_slot:
	.zero 8
rt_qpc_freq:
	.zero 8
rt_heap_committed:
	.zero 8
rt_poll_event:
	.zero 8
rt_poll_lock:
	.zero 8
rt_ready_list:
	.zero 8

	.text
rt_write:
	push rbx
	push r12
	push r13
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 64
	mov r12, rsi
	mov r13, rdx
	mov ecx, -11
	cmp edi, 2
	jne 1f
	mov ecx, -12
1:
	WIN GetStdHandle
	mov rbx, rax
2:
	test r13, r13
	jz 3f
	mov rcx, rbx
	mov rdx, r12
	mov r8, r13
	mov eax, 0x40000000
	cmp r8, rax
	cmova r8, rax
	lea r9, [rsp+48]
	mov dword ptr [r9], 0
	mov qword ptr [rsp+32], 0
	WIN WriteFile
	test eax, eax
	jz 3f
	mov eax, dword ptr [rsp+48]
	test eax, eax
	jz 3f
	add r12, rax
	sub r13, rax
	jmp 2b
3:
	mov rsp, rbp
	pop rbp
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_print_str
rt_print_str:
	cmp rsi, 2047
	ja 1f
	push rbp
	mov rbp, rsp
	sub rsp, 2048
	and rsp, -16
	mov rcx, rsi
	mov rsi, rdi
	mov rdi, rsp
	rep movsb
	mov byte ptr [rdi], 10
	lea rdx, [rdi+1]
	sub rdx, rsp
	mov rsi, rsp
	mov edi, 1
	call rt_write
	mov rsp, rbp
	pop rbp
	ret
1:
	mov rdx, rsi
	mov rsi, rdi
	mov edi, 1
	call rt_write
	mov edi, 1
	lea rsi, [rip+rt_nl]
	mov edx, 1
	jmp rt_write

rt_exit:
	mov ecx, edi
	and rsp, -16
	sub rsp, 32
	WIN ExitProcess
	ud2

rt_env_byte:
	OS_ENTER
	mov rcx, rdi
	lea rdx, [rsp+48]
	mov r8d, 32
	WIN GetEnvironmentVariableA
	xor ecx, ecx
	test eax, eax
	jz 1f
	cmp eax, 32
	jae 1f
	movzx ecx, byte ptr [rsp+48]
1:
	mov eax, ecx
	OS_LEAVE
	ret

rt_mmap:
	OS_ENTER
	xor ecx, ecx
	mov rdx, rsi
	mov r8d, MEM_RESERVE
	mov r9d, PAGE_READWRITE
	WIN VirtualAlloc
	OS_LEAVE
	test rax, rax
	jz rt_panic_oom
	ret

rt_mmap_sparse:
	jmp rt_mmap

rt_heap_release:
	ret

rt_mmap_stack:
	OS_ENTER
	xor ecx, ecx
	mov rdx, rsi
	mov r8d, MEM_COMMIT|MEM_RESERVE
	mov r9d, PAGE_READWRITE
	WIN VirtualAlloc
	OS_LEAVE
	test rax, rax
	jz rt_panic_oom
	ret

rt_commit:
	OS_ENTER
	mov r8d, MEM_COMMIT
	mov r9d, PAGE_READWRITE
	WIN VirtualAlloc
	OS_LEAVE
	test rax, rax
	jz rt_panic_oom
	ret

rt_region_commit:
	mov rcx, rdi
	mov rdx, rsi
	jmp rt_commit

rt_region_stash:
rt_region_decommit:
	OS_ENTER
	mov rcx, rdi
	mov rdx, rsi
	mov r8d, MEM_DECOMMIT
	WIN VirtualFree
	OS_LEAVE
	ret

rt_heap_commit:
	push rbx
	push r12
	push r13
	mov rbx, [rip+rt_heap_start]
	mov r12, [rip+rt_heap_committed]
	test r12, r12
	cmovz r12, rbx
	lea r13, [rdi+HEAP_CHUNK-1]
	and r13, -HEAP_CHUNK
	cmp r13, [rip+rt_heap_end]
	cmova r13, [rip+rt_heap_end]
	mov rcx, r12
	mov rdx, r13
	sub rdx, r12
	call rt_commit
	mov rcx, r12
	sub rcx, rbx
	shr rcx, 6
	add rcx, [rip+rt_bitmap]
	mov rdx, r13
	sub rdx, r12
	shr rdx, 6
	add rdx, 8
	call rt_commit
	mov rcx, r12
	sub rcx, rbx
	shr rcx, 6
	add rcx, [rip+rt_mark_bits]
	mov rdx, r13
	sub rdx, r12
	shr rdx, 6
	add rdx, 8
	call rt_commit
	mov rcx, r12
	sub rcx, rbx
	shr rcx, 12
	add rcx, [rip+rt_summary1]
	mov rdx, r13
	sub rdx, r12
	shr rdx, 12
	add rdx, 8
	call rt_commit
	mov rcx, r12
	sub rcx, rbx
	shr rcx, 18
	add rcx, [rip+rt_summary2]
	mov rdx, r13
	sub rdx, r12
	shr rdx, 18
	add rdx, 8
	call rt_commit
	mov rcx, r12
	sub rcx, rbx
	shr rcx, 15
	add rcx, [rip+rt_dirty]
	mov rdx, r13
	sub rdx, r12
	shr rdx, 15
	add rdx, 8
	call rt_commit
	mov rcx, r12
	sub rcx, rbx
	shr rcx, 1
	add rcx, [rip+rt_mark_base]
	mov rdx, r13
	sub rdx, r12
	shr rdx, 1
	call rt_commit
	mov [rip+rt_heap_committed], r13
	pop r13
	pop r12
	pop rbx
	ret

rt_yield:
	OS_ENTER
	WIN SwitchToThread
	OS_LEAVE
	ret

rt_futex_wait:
	OS_ENTER
	mov [rsp+48], esi
	mov rcx, rdi
	lea rdx, [rsp+48]
	mov r8d, 4
	mov r9d, -1
	WIN WaitOnAddress
	OS_LEAVE
	ret

rt_futex_wake:
	OS_ENTER
	mov rcx, rdi
	cmp esi, 1
	jne 1f
	WIN WakeByAddressSingle
	jmp 2f
1:
	WIN WakeByAddressAll
2:
	OS_LEAVE
	ret

rt_os_stack:
	OS_ENTER
	xor ecx, ecx
	mov edx, G_MAP
	mov r8d, MEM_RESERVE
	mov r9d, PAGE_READWRITE
	WIN VirtualAlloc
	test rax, rax
	jz 1f
	mov rbx, rax
	lea rcx, [rbx+G_MAP-STACK_COMMIT]
	mov edx, STACK_COMMIT
	mov r8d, MEM_COMMIT
	mov r9d, PAGE_READWRITE
	WIN VirtualAlloc
	test rax, rax
	jz 1f
	lea rcx, [rbx+G_MAP-STACK_COMMIT-4096]
	mov edx, 4096
	mov r8d, MEM_COMMIT
	mov r9d, PAGE_GUARDED
	WIN VirtualAlloc
	test rax, rax
	jz 1f
	lea rax, [rbx+G_MAP-STACK_COMMIT]
	mov [rbx+G_RECORD+G_LIMIT], rax
	mov rax, rbx
	OS_LEAVE
	ret
1:
	jmp rt_panic_oom

rt_os_m_start:
	OS_ENTER
	xor ecx, ecx
	mov edx, G0_STACK
	lea r8, [rip+rt_mstart]
	mov r9, rbx
	mov qword ptr [rsp+32], STACK_RESERVATION
	mov qword ptr [rsp+40], 0
	WIN CreateThread
	test rax, rax
	jz 1f
	mov rcx, rax
	WIN CloseHandle
1:
	OS_LEAVE
	ret

rt_mstart:
	mov rbx, rcx
	call rt_os_tls
	and rsp, -16
	mov [rbx+M_G0TOP], rsp
	mov [rbx+M_G0SP], rsp
	jmp rt_schedule

rt_os_init:
	OS_ENTER
	lea rcx, [rsp+48]
	WIN GetSystemInfo
	mov eax, dword ptr [rsp+80]
	test eax, eax
	jnz 1f
	mov eax, 1
1:
	mov [rip+rt_ncpu], rax
	WIN TlsAlloc
	cmp eax, TLS_SLOT_COUNT
	jae 2f
	lea rax, [8*rax+TEB_TLS_SLOTS]
	mov [rip+rt_tls_slot], rax
	lea rcx, [rip+rt_qpc_freq]
	WIN QueryPerformanceFrequency
	OS_LEAVE
	ret
2:
	lea rdi, [rip+rt_msg_tls]
	jmp rt_panic

rt_os_tls:
	call rt_tcb_init
	mov rcx, [rip+rt_tls_slot]
	mov qword ptr gs:[rcx], rax
	mov rax, qword ptr gs:[TEB_STACK_BASE]
	mov [rbx+M_OSBASE], rax
	mov rax, qword ptr gs:[TEB_STACK_LIMIT]
	mov [rbx+M_OSLIMIT], rax
	mov rax, qword ptr gs:[TEB_DEALLOCATION_STACK]
	mov [rbx+M_OSDEALLOC], rax
	ret

rt_os_signals:
	OS_ENTER
	mov ecx, 1
	lea rdx, [rip+rt_fault]
	WIN AddVectoredExceptionHandler
	OS_LEAVE
	ret

rt_fault:
	mov r8, [rip+rt_tls_slot]
	mov r8, qword ptr gs:[r8]
	test r8, r8
	jz 9f
	mov rax, [rcx]
	mov edx, dword ptr [rax]
	lea r9, [rip+rt_msg_overflow]
	cmp edx, EXCEPTION_STACK_OVERFLOW
	je 2f
	cmp edx, EXCEPTION_ACCESS_VIOLATION
	jne 9f
	lea r9, [rip+rt_msg_fault]
	mov r10, [r8+TLS_G]
	test r10, r10
	jz 2f
	mov r11, [rax+EXCEPTION_ADDRESS]
	cmp r11, [r10+G_LO]
	jb 3f
	cmp r11, [r10+G_HI]
	jae 3f
	lea r9, [rip+rt_msg_overflow]
	jmp 2f
3:
	sub r11, [rip+rt_region_start]
	shr r11, REGION_BITS
	jnz 2f
	lea r9, [rip+rt_msg_region_freed]
2:
	mov rax, [rcx+8]
	mov [rax+CONTEXT_RDI], r9
	lea rdx, [rip+rt_fault_panic]
	mov [rax+CONTEXT_RIP], rdx
	mov r10, [r8+TLS_M]
	mov rdx, [r10+M_G0SP]
	and rdx, -16
	mov [rax+CONTEXT_RSP], rdx
	mov eax, -1
	ret
9:
	xor eax, eax
	ret

rt_fault_panic:
	TLS_GET rax, TLS_M
	TEB_OS rax
	jmp rt_panic

rt_os_sysmon_start:
	OS_ENTER
	xor ecx, ecx
	mov edx, 65536
	lea r8, [rip+rt_sysmon]
	xor r9d, r9d
	mov qword ptr [rsp+32], STACK_RESERVATION
	mov qword ptr [rsp+40], 0
	WIN CreateThread
	test rax, rax
	jz 1f
	mov rcx, rax
	WIN CloseHandle
1:
	OS_LEAVE
	ret

rt_os_nap:
	OS_ENTER
	mov ecx, 10
	WIN Sleep
	OS_LEAVE
	ret

rt_qpc_nanos:
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 48
	lea rcx, [rsp+32]
	WIN QueryPerformanceCounter
	mov rax, [rsp+32]
	mov rcx, 1000000000
	mul rcx
	div qword ptr [rip+rt_qpc_freq]
	mov rsp, rbp
	pop rbp
	ret

rt_poll_init:
	cmp qword ptr [rip+rt_poll_event], 0
	je 1f
	ret
1:
	lea rdi, [rip+rt_poll_lock]
	call rt_spin_lock
	cmp qword ptr [rip+rt_poll_event], 0
	jne 2f
	OS_ENTER
	xor ecx, ecx
	xor edx, edx
	xor r8d, r8d
	xor r9d, r9d
	WIN CreateEventW
	mov [rip+rt_poll_event], rax
	OS_LEAVE
2:
	lea rdi, [rip+rt_poll_lock]
	jmp rt_spin_unlock

rt_poll_wake:
	mfence
	cmp qword ptr [rip+rt_poller], 0
	je 1f
	mov rcx, [rip+rt_poll_event]
	test rcx, rcx
	jz 1f
	OS_ENTER
	WIN SetEvent
	OS_LEAVE
1:
	ret

rt_netpoll:
	cmp qword ptr [rip+rt_poll_event], 0
	jne 1f
	xor eax, eax
	ret
1:
	push rbx
	push r12
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 32
	test rdi, rdi
	jz 2f
	mov rcx, [rip+rt_poll_event]
	mov edx, edi
	WIN WaitForSingleObject
2:
	xor r12d, r12d
	xor ebx, ebx
	xchg [rip+rt_ready_list], rbx
3:
	test rbx, rbx
	jz 4f
	mov rdi, [rbx+8]
	mov rbx, [rbx]
	lock dec qword ptr [rip+rt_nfdwait]
	call rt_ready
	inc r12
	jmp 3b
4:
	mov rax, r12
	mov rsp, rbp
	pop rbp
	pop r12
	pop rbx
	ret

rt_wait_done:
	sub rsp, 40
	mov rax, [rip+rt_ready_list]
1:
	mov [rcx], rax
	lock cmpxchg [rip+rt_ready_list], rcx
	jne 1b
	mov rcx, [rip+rt_poll_event]
	WIN SetEvent
	add rsp, 40
	ret

	.globl rt_wait_readable
	.globl rt_wait_writable
rt_wait_readable:
rt_wait_writable:
	ENTER_RT
	push rbx
	push rdi
	call rt_poll_init
	pop rdi
	lock inc qword ptr [rip+rt_nfdwait]
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 80
	lea rbx, [rsp+48]
	mov qword ptr [rbx], 0
	TLS_GET rax, TLS_G
	mov [rbx+8], rax
	lea rcx, [rbx+16]
	mov rdx, rdi
	lea r8, [rip+rt_wait_done]
	mov r9, rbx
	mov qword ptr [rsp+32], -1
	mov qword ptr [rsp+40], WT_EXECUTEONLYONCE
	WIN RegisterWaitForSingleObject
	test eax, eax
	jz 1f
	call rt_park_body
	mov rcx, [rbx+16]
	WIN UnregisterWait
	jmp 2f
1:
	lock dec qword ptr [rip+rt_nfdwait]
2:
	mov rsp, rbp
	pop rbp
	pop rbx
	ret

	.section .rodata
	MSG_BEGIN rt_msg_syscall
	.ascii "system calls are not available on windows"
	MSG_END

	.text
rt_panic_syscall:
	lea rdi, [rip+rt_msg_syscall]
	jmp rt_panic

	.globl rt_sysblock
rt_sysblock:
	jmp rt_panic_syscall

rt_seed_init:
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 48
	lea rcx, [rip+rt_hash_seed]
	mov edx, 16
	WIN ProcessPrng
	test eax, eax
	jnz 9f
	call rt_seed_fallback
	mov [rip+rt_hash_seed], rax
	mov [rip+rt_hash_seed+8], rdx
9:
	or qword ptr [rip+rt_hash_seed+8], 1
	mov rsp, rbp
	pop rbp
	ret

rt_vdso_init:
	ret

	.globl rt_clock_nanos
rt_clock_nanos:
	cmp rdi, CLOCK_MONOTONIC
	je rt_qpc_nanos
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 48
	lea rcx, [rsp+32]
	WIN GetSystemTimePreciseAsFileTime
	mov rax, [rsp+32]
	mov rcx, 116444736000000000
	sub rax, rcx
	imul rax, rax, 100
	mov rsp, rbp
	pop rbp
	ret
