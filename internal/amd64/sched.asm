
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
	.set M_RUNNEXT, 128
	.set M_SPINNING, 136
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
	mov rax, qword ptr fs:[TLS_G]
	mov r11, [rsp]
	mov [rax+G_PC], r11
	lea r11, [rsp+8]
	mov [rax+G_USERSP], r11
	mov [rax+G_FP], rbp
	.endm

	.section .tbss,"awT",@nobits
	.p2align 3
rt_tls:
	.zero 24

	.section .rodata
rt_msg_deadlock:
	.quad 37
	.ascii "every green thread is blocked forever"
rt_msg_overflow:
	.quad 14
	.ascii "stack overflow"
rt_msg_fault:
	.quad 21
	.ascii "invalid memory access"
rt_msg_timers:
	.quad 15
	.ascii "too many timers"
rt_msg_threads:
	.quad 22
	.ascii "too many green threads"
rt_sysmon_sleep:
	.quad 0
	.quad 10000000

	.bss
	.p2align 3
	.globl rt_stop
rt_stop:
	.zero 8
rt_sched_lock:
	.zero 8
rt_nspinning:
	.zero 8
rt_allm:
	.zero 8*MAXM
rt_nm:
	.zero 8
rt_ncpu:
	.zero 8
rt_idle:
	.zero 8
rt_nidle:
	.zero 8
rt_gq_head:
	.zero 8
rt_gq_tail:
	.zero 8
rt_gq_n:
	.zero 8
rt_allg_lock:
	.zero 8
rt_allgs:
	.zero 8
rt_ngs:
	.zero 8
rt_gfree:
	.zero 8
rt_sysmon_on:
	.zero 8
rt_timer_lock:
	.zero 8
rt_timers:
	.zero 8
rt_ntimers:
	.zero 8
rt_timer_seq:
	.zero 8
rt_epfd:
	.zero 8
rt_evfd:
	.zero 8
rt_poller:
	.zero 8
rt_nfdwait:
	.zero 8
rt_poll_lock:
	.zero 8
rt_scratch:
	.zero 128

	.text
rt_spin_lock:
	mov eax, 1
	xchg [rdi], rax
	test rax, rax
	jz 2f
	xor ecx, ecx
1:
	pause
	inc ecx
	cmp ecx, 1000
	jb 3f
	push rdi
	mov eax, 24
	syscall
	pop rdi
	xor ecx, ecx
3:
	cmp qword ptr [rdi], 0
	jne 1b
	jmp rt_spin_lock
2:
	ret

rt_spin_unlock:
	mov qword ptr [rdi], 0
	ret

rt_futex_wait:
	mov eax, 202
	mov rdx, rsi
	mov esi, 128
	xor r10d, r10d
	syscall
	ret

rt_futex_wake:
	mov eax, 202
	mov rdx, rsi
	mov esi, 129
	syscall
	ret

rt_gc_wait:
	push rdi
1:
	cmp dword ptr [rip+rt_stop], 0
	je 2f
	lea rdi, [rip+rt_stop]
	mov esi, 1
	call rt_futex_wait
	jmp 1b
2:
	pop rdi
	ret

	.globl rt_m_running
rt_m_running:

	push rbx
	push r12
	mov rbx, qword ptr fs:[TLS_M]
	mov r12, rdi
1:
	mov rax, r12
	mov ecx, RUNNING
	lock cmpxchg [rbx+M_STATE], rcx
	je 2f
	call rt_gc_wait
	pause
	jmp 1b
2:
	cmp dword ptr [rip+rt_stop], 0
	je 3f
	mov eax, RUNNING
	mov rcx, r12
	lock cmpxchg [rbx+M_STATE], rcx
	call rt_gc_wait
	jmp 1b
3:
	pop r12
	pop rbx
	ret

	.globl rt_exitc
rt_exitc:
	push rax
	push rdx
	push rdi
	push rsi
	push rcx
	push r8
	push r9
	push r10
	push r11
	sub rsp, 24
	movq [rsp], xmm0
	movq [rsp+8], xmm1
	mov edi, INC
	call rt_m_running
	movq xmm0, [rsp]
	movq xmm1, [rsp+8]
	add rsp, 24
	pop r11
	pop r10
	pop r9
	pop r8
	pop rcx
	pop rsi
	pop rdi
	pop rdx
	pop rax
	ret

rt_stop_here:
	SAVE_ALL
	mov rax, qword ptr fs:[TLS_G]
	mov [rax+G_SCANLO], rsp
	mov rbx, qword ptr fs:[TLS_M]
	mov eax, STOPPED
	xchg [rbx+M_STATE], rax
	mov edi, STOPPED
	call rt_m_running
	RESTORE_ALL
	ret

rt_set_idle:
	mov rcx, qword ptr fs:[TLS_M]
	mov qword ptr [rcx+M_CURG], 0
	mov qword ptr fs:[TLS_G], 0
	mov eax, IDLE
	xchg [rcx+M_STATE], rax
	ret

	.globl rt_lock
rt_lock:
	ENTER_RT
rt_lock_body:
	xor eax, eax
	mov ecx, 1
	lock cmpxchg [rdi], rcx
	jz 9f
	mov r8d, 100
1:
	cmp dword ptr [rip+rt_stop], 0
	je 2f
	push rdi
	call rt_stop_here
	pop rdi
2:
	pause
	cmp qword ptr [rdi], 0
	jne 3f
	xor eax, eax
	mov ecx, 1
	lock cmpxchg [rdi], rcx
	jz 9f
3:
	dec r8d
	jnz 1b
4:
	mov eax, 2
	xchg [rdi], rax
	test rax, rax
	jz 9f
	push rdi
	call rt_lock_sleep
	pop rdi
	jmp 4b
9:
	mov rax, qword ptr fs:[TLS_M]
	inc qword ptr [rax+M_LOCKS]
	ret

rt_lock_sleep:
	SAVE_ALL
	mov rax, qword ptr fs:[TLS_G]
	mov [rax+G_SCANLO], rsp
	mov rbx, qword ptr fs:[TLS_M]
	mov eax, STOPPED
	xchg [rbx+M_STATE], rax
	mov esi, 2
	call rt_futex_wait
	mov edi, STOPPED
	call rt_m_running
	RESTORE_ALL
	ret

	.globl rt_unlock
rt_unlock:
	mov rax, qword ptr fs:[TLS_M]
	dec qword ptr [rax+M_LOCKS]
	xor eax, eax
	xchg [rdi], rax
	cmp rax, 2
	je 1f
	ret
1:
	mov esi, 1
	jmp rt_futex_wake

	.globl rt_lock_all
rt_lock_all:
	ENTER_RT
	push rbx
	push r12
	push r13
	mov rbx, rdi
	mov r12, rsi
	mov ecx, 1
1:
	cmp rcx, r12
	jae 4f
	mov rax, [rbx+8*rcx]
	mov rdx, rcx
2:
	test rdx, rdx
	jz 3f
	cmp [rbx+8*rdx-8], rax
	jbe 3f
	mov r8, [rbx+8*rdx-8]
	mov [rbx+8*rdx], r8
	dec rdx
	jmp 2b
3:
	mov [rbx+8*rdx], rax
	inc rcx
	jmp 1b
4:
	xor r13d, r13d
5:
	cmp r13, r12
	jae 7f
	mov rdi, [rbx+8*r13]
	test r13, r13
	jz 6f
	cmp rdi, [rbx+8*r13-8]
	je 8f
6:
	call rt_lock_body
8:
	inc r13
	jmp 5b
7:
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_unlock_all
rt_unlock_all:
	push rbx
	push r12
	push r13
	mov rbx, rdi
	mov r12, rsi
	xor r13d, r13d
1:
	cmp r13, r12
	jae 3f
	mov rdi, [rbx+8*r13]
	test r13, r13
	jz 2f
	cmp rdi, [rbx+8*r13-8]
	je 4f
2:
	call rt_unlock
4:
	inc r13
	jmp 1b
3:
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_cas
rt_cas:
	mov rax, rsi
	lock cmpxchg [rdi], rdx
	sete al
	movzx eax, al
	ret

	.globl rt_current
rt_current:
	mov rax, qword ptr fs:[TLS_G]
	ret

	.globl rt_context
rt_context:
	mov rax, qword ptr fs:[TLS_G]
	mov rax, [rax+G_CTX]
	ret

	.globl rt_set_context
rt_set_context:
	mov rax, qword ptr fs:[TLS_G]
	mov [rax+G_CTX], rdi
	ret

	.globl rt_has_context
rt_has_context:
	mov rax, qword ptr fs:[TLS_G]
	cmp qword ptr [rax+G_CTX], 0
	setne al
	movzx eax, al
	ret

rt_mmap_stack:
	mov edx, 3
	mov r10d, 0x4022
	mov r8, -1
	xor r9d, r9d
	xor edi, edi
	mov eax, 9
	syscall
	cmp rax, -4096
	ja rt_panic_oom
	ret

rt_galloc:
	push rbx
	lea rdi, [rip+rt_allg_lock]
	call rt_spin_lock
	mov rax, [rip+rt_gfree]
	test rax, rax
	jz 1f
	mov rcx, [rax+G_NEXT]
	mov [rip+rt_gfree], rcx
	lea rdi, [rip+rt_allg_lock]
	mov rbx, rax
	call rt_spin_unlock
	mov rax, rbx
	pop rbx
	ret
1:
	lea rdi, [rip+rt_allg_lock]
	call rt_spin_unlock
	mov rsi, G_MAP
	call rt_mmap_stack
	mov rbx, rax
	mov rdi, rax
	mov esi, GUARD
	xor edx, edx
	mov eax, 10
	syscall
	lea rax, [rbx+GUARD+STACK]
	mov [rax+G_HI], rax
	mov [rax+G_LO], rbx
	mov rbx, rax
	lea rdi, [rip+rt_allg_lock]
	call rt_spin_lock
	mov rcx, [rip+rt_ngs]
	cmp rcx, MAXG
	jae 2f
	mov rdx, [rip+rt_allgs]
	mov [rdx+8*rcx], rbx
	inc rcx
	mov [rip+rt_ngs], rcx
	lea rdi, [rip+rt_allg_lock]
	call rt_spin_unlock
	mov rax, rbx
	pop rbx
	ret
2:
	lea rdi, [rip+rt_msg_threads]
	jmp rt_panic

rt_gq_put:
	push rbx
	mov rbx, rdi
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_lock
	mov qword ptr [rbx+G_NEXT], 0
	mov rax, [rip+rt_gq_tail]
	test rax, rax
	jz 1f
	mov [rax+G_NEXT], rbx
	jmp 2f
1:
	mov [rip+rt_gq_head], rbx
2:
	mov [rip+rt_gq_tail], rbx
	inc qword ptr [rip+rt_gq_n]
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	pop rbx
	ret

rt_gq_get:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_lock
	mov rax, [rip+rt_gq_head]
	test rax, rax
	jz 1f
	mov rcx, [rax+G_NEXT]
	mov [rip+rt_gq_head], rcx
	test rcx, rcx
	jnz 2f
	mov qword ptr [rip+rt_gq_tail], 0
2:
	dec qword ptr [rip+rt_gq_n]
1:
	push rax
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	pop rax
	ret

rt_runq_put:
	push rbx
	push r12
	mov r12, rdi
	mov rbx, qword ptr fs:[TLS_M]
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_lock
	mov rax, [rbx+M_QTAIL]
	mov rcx, rax
	sub rcx, [rbx+M_QHEAD]
	cmp rcx, 256
	jae 1f
	mov rcx, rax
	and ecx, 255
	mov [rbx+M_QUEUE+8*rcx], r12
	inc rax
	mov [rbx+M_QTAIL], rax
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_unlock
	pop r12
	pop rbx
	ret
1:
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_unlock
	mov rdi, r12
	pop r12
	pop rbx
	jmp rt_gq_put

rt_runq_get:
	push rbx
	mov rbx, rdi
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_lock
	xor eax, eax
	mov rcx, [rbx+M_QHEAD]
	cmp rcx, [rbx+M_QTAIL]
	je 1f
	mov rdx, rcx
	and edx, 255
	mov rax, [rbx+M_QUEUE+8*rdx]
	inc rcx
	mov [rbx+M_QHEAD], rcx
1:
	push rax
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_unlock
	pop rax
	pop rbx
	ret

rt_steal:
	push rbx
	push r12
	xor r12d, r12d
1:
	cmp r12, [rip+rt_nm]
	jae 2f
	lea rax, [rip+rt_allm]
	mov rdi, [rax+8*r12]
	inc r12
	cmp rdi, qword ptr fs:[TLS_M]
	je 1b
	mov rax, [rdi+M_QHEAD]
	cmp rax, [rdi+M_QTAIL]
	je 1b
	call rt_runq_get
	test rax, rax
	jz 1b
	pop r12
	pop rbx
	ret
2:
	xor eax, eax
	pop r12
	pop rbx
	ret

rt_queues_empty:
	cmp qword ptr [rip+rt_gq_head], 0
	jne 2f
	xor ecx, ecx
1:
	cmp rcx, [rip+rt_nm]
	jae 3f
	lea rax, [rip+rt_allm]
	mov rdx, [rax+8*rcx]
	mov rax, [rdx+M_QHEAD]
	cmp rax, [rdx+M_QTAIL]
	jne 2f
	inc rcx
	jmp 1b
2:
	xor eax, eax
	ret
3:
	mov eax, 1
	ret

	.globl rt_ready
rt_ready:
	push rbx
	mov rbx, rdi
	lea rdi, [rbx+G_LOCK]
	call rt_spin_lock
	cmp qword ptr [rbx+G_STATUS], G_WAITING
	jne 1f
	mov qword ptr [rbx+G_STATUS], G_RUNNABLE
	lea rdi, [rbx+G_LOCK]
	call rt_spin_unlock
	cmp qword ptr [rbx+G_LOCKEDM], 0
	jne 2f
	mov rax, qword ptr fs:[TLS_M]
	mov rdi, rbx
	xchg [rax+M_RUNNEXT], rdi
	test rdi, rdi
	jz 3f
	call rt_runq_put
	call rt_wakep
3:
	pop rbx
	ret
2:
	lea rdi, [rbx+G_STATUS]
	mov esi, 1
	call rt_futex_wake
	pop rbx
	ret
1:
	mov qword ptr [rbx+G_TOKEN], 1
	lea rdi, [rbx+G_LOCK]
	call rt_spin_unlock
	pop rbx
	ret

rt_wakep:
	cmp qword ptr [rip+rt_nspinning], 0
	je 7f
	ret
7:
	push rbx
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_lock
	mov rbx, [rip+rt_idle]
	test rbx, rbx
	jz 1f
	mov rax, [rbx+M_NEXT]
	mov [rip+rt_idle], rax
	dec qword ptr [rip+rt_nidle]
	mov qword ptr [rbx+M_SPINNING], 1
	lock inc qword ptr [rip+rt_nspinning]
	mov qword ptr [rbx+M_PARK], 1
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	lea rdi, [rbx+M_PARK]
	mov esi, 1
	call rt_futex_wake
	pop rbx
	ret
1:
	xor ecx, ecx
	xor edx, edx
2:
	cmp rcx, [rip+rt_nm]
	jae 3f
	lea rax, [rip+rt_allm]
	mov rax, [rax+8*rcx]
	mov rax, [rax+M_STATE]
	and eax, 7
	cmp eax, INC
	je 4f
	inc rdx
4:
	inc rcx
	jmp 2b
3:
	cmp rdx, [rip+rt_ncpu]
	jae 5f
	cmp qword ptr [rip+rt_nm], MAXM
	jae 5f
	call rt_newm
	pop rbx
	ret
5:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	cmp qword ptr [rip+rt_poller], 0
	je 6f
	call rt_poll_wake
6:
	pop rbx
	ret

rt_newm:
	push rbx
	push r12
	mov esi, M_SIZE
	call rt_mmap_stack
	mov rbx, rax
	mov qword ptr [rbx+M_STATE], IDLE
	mov rsi, G0_STACK
	call rt_mmap_stack
	add rax, G0_STACK
	mov [rbx+M_G0TOP], rax
	mov [rbx+M_G0SP], rax
	lea rax, [rbx+M_TCB]
	mov [rax], rax
	mov [rax+TLS_M], rbx
	lea rcx, [rax+TLS_FLAG]
	mov [rbx+M_FLAG], rcx
	mov rcx, [rip+rt_nm]
	lea rax, [rip+rt_allm]
	mov [rax+8*rcx], rbx
	inc rcx
	mov [rip+rt_nm], rcx
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
.if RT_DYNAMIC
	push r13
	mov r13, rsp
	and rsp, -16
	sub rsp, 16
	mov rdi, rsp
	xor esi, esi
	lea rdx, [rip+rt_mstart_c]
	mov rcx, rbx
	call pthread_create
	mov rsp, r13
	pop r13
.else
	mov r12, rbx
	mov edi, 0xD0F00
	mov rsi, [rbx+M_G0TOP]
	xor edx, edx
	xor r10d, r10d
	lea r8, [rbx+M_TCB]
	mov eax, 56
	syscall
	test rax, rax
	jnz 1f
	mov rbx, r12
	jmp rt_mstart
1:
.endif
	pop r12
	pop rbx
	ret

.if RT_DYNAMIC
rt_mstart_c:
	mov rbx, rdi
	mov qword ptr fs:[TLS_M], rbx
	mov rax, qword ptr fs:[0]
	add rax, TLS_FLAG
	mov [rbx+M_FLAG], rax
	and rsp, -16
	mov [rbx+M_G0TOP], rsp
	mov [rbx+M_G0SP], rsp
.endif
rt_mstart:
	call rt_altstack
	mov rsp, [rbx+M_G0TOP]
	jmp rt_schedule

rt_altstack:
	mov esi, 65536
	call rt_mmap_stack
	sub rsp, 32
	mov [rsp], rax
	mov dword ptr [rsp+8], 0
	mov qword ptr [rsp+16], 65536
	mov rdi, rsp
	xor esi, esi
	mov eax, 131
	syscall
	add rsp, 32
	ret

	.globl rt_sched_init
rt_sched_init:
	push rbx
	sub rsp, 128
	mov rdi, rsp
	xor eax, eax
	mov ecx, 16
	rep stosq
	xor edi, edi
	mov esi, 128
	mov rdx, rsp
	mov eax, 204
	syscall
	xor ecx, ecx
	xor edx, edx
1:
	cmp rcx, 16
	jae 2f
	popcnt r8, [rsp+8*rcx]
	add rdx, r8
	inc rcx
	jmp 1b
2:
	test rdx, rdx
	jnz 3f
	mov edx, 1
3:
	mov [rip+rt_ncpu], rdx
	add rsp, 128
	mov rsi, 8*MAXG
	call rt_mmap_stack
	mov [rip+rt_allgs], rax
	mov rsi, 8*T_WORDS*MAXT
	call rt_mmap_stack
	mov [rip+rt_timers], rax
	mov esi, M_SIZE
	call rt_mmap_stack
	mov rbx, rax
	mov qword ptr [rbx+M_STATE], IDLE
	lea rax, [rip+rt_allm]
	mov [rax], rbx
	mov qword ptr [rip+rt_nm], 1
.if RT_DYNAMIC
	mov qword ptr fs:[TLS_M], rbx
	mov rax, qword ptr fs:[0]
	add rax, TLS_FLAG
	mov [rbx+M_FLAG], rax
.else
	lea rsi, [rbx+M_TCB]
	mov [rsi], rsi
	mov [rsi+TLS_M], rbx
	lea rax, [rsi+TLS_FLAG]
	mov [rbx+M_FLAG], rax
	mov edi, 0x1002
	mov eax, 158
	syscall
.endif
	lea rax, [rsp+16]
	and rax, -16
	mov [rbx+M_G0TOP], rax
	mov [rbx+M_G0SP], rax
	call rt_altstack
	call rt_fault_init
	pop rbx
	ret

rt_fault_init:
	sub rsp, 32
	lea rax, [rip+rt_fault]
	mov [rsp], rax
	mov qword ptr [rsp+8], 0x0C000004
	lea rax, [rip+rt_restorer]
	mov [rsp+16], rax
	mov qword ptr [rsp+24], 0
	mov edi, 11
	mov rsi, rsp
	xor edx, edx
	mov r10d, 8
	mov eax, 13
	syscall
	mov edi, 7
	mov rsi, rsp
	xor edx, edx
	mov r10d, 8
	mov eax, 13
	syscall
	add rsp, 32
	ret

rt_restorer:
	mov eax, 15
	syscall

rt_fault:
	mov rax, [rsi+16]
	mov rcx, qword ptr fs:[TLS_G]
	test rcx, rcx
	jz 1f
	mov rdx, [rcx+G_LO]
	cmp rax, rdx
	jb 1f
	add rdx, GUARD
	cmp rax, rdx
	jae 1f
	lea rdi, [rip+rt_msg_overflow]
	jmp rt_panic
1:
	lea rdi, [rip+rt_msg_fault]
	jmp rt_panic

	.globl rt_spawn
rt_spawn:
	push rbx
	push r12
	push r13
	mov r12, rdi
	mov r13, rsi
	call rt_galloc
	mov rbx, rax
	mov rcx, [rbx+G_HI]
	lea rdi, [rcx-128]
	xor eax, eax
	mov ecx, 15
	rep stosq
	mov rcx, [rbx+G_HI]
	lea rax, [rip+rt_gentry]
	mov [rcx-8], rax
	mov [rcx-16], r12
	mov [rcx-56], r13
	lea rax, [rcx-128]
	mov [rbx+G_SP], rax
	mov [rbx+G_SCANLO], rax
	mov qword ptr [rbx+G_PC], 0
	mov qword ptr [rbx+G_FP], 0
	mov qword ptr [rbx+G_TOKEN], 0
	mov qword ptr [rbx+G_LOCK], 0
	mov qword ptr [rbx+G_LOCKEDM], 0
	mov rax, qword ptr fs:[TLS_G]
	test rax, rax
	jz 1f
	mov rax, [rax+G_CTX]
1:
	mov [rbx+G_CTX], rax
	mov qword ptr [rbx+G_STATUS], G_RUNNABLE
	mov rdi, rbx
	call rt_runq_put
	call rt_wakep
	call rt_sysmon_start
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_spawn_main
rt_spawn_main:
	push rbx
	push r12
	push r13
	mov r12, rdi
	xor r13d, r13d
	call rt_galloc
	mov rbx, rax
	mov rcx, [rbx+G_HI]
	lea rdi, [rcx-128]
	xor eax, eax
	mov ecx, 15
	rep stosq
	mov rcx, [rbx+G_HI]
	lea rax, [rip+rt_gentry]
	mov [rcx-8], rax
	mov [rcx-16], r12
	lea rax, [rcx-128]
	mov [rbx+G_SP], rax
	mov [rbx+G_SCANLO], rax
	mov qword ptr [rbx+G_CTX], 0
	mov qword ptr [rbx+G_STATUS], G_RUNNABLE
	mov rdi, rbx
	call rt_gq_put
	pop r13
	pop r12
	pop rbx
	ret

rt_gentry:
	xor ebp, ebp
	call rax
	mov rdi, qword ptr fs:[TLS_G]
	mov rbx, qword ptr fs:[TLS_M]
	mov rsp, [rbx+M_G0TOP]
	mov qword ptr [rdi+G_STATUS], G_FREE
	mov r12, rdi
	lea rdi, [rip+rt_allg_lock]
	call rt_spin_lock
	mov rax, [rip+rt_gfree]
	mov [r12+G_NEXT], rax
	mov [rip+rt_gfree], r12
	lea rdi, [rip+rt_allg_lock]
	call rt_spin_unlock
	call rt_set_idle
	jmp rt_schedule

	.globl rt_park
rt_park:
	ENTER_RT
rt_park_body:
	SAVE_ALL
	mov r12, qword ptr fs:[TLS_G]
	mov [r12+G_SP], rsp
	mov [r12+G_SCANLO], rsp
	mov rbx, qword ptr fs:[TLS_M]
	mov rsp, [rbx+M_G0SP]
	lea rdi, [r12+G_LOCK]
	call rt_spin_lock
	cmp qword ptr [r12+G_TOKEN], 0
	jne 1f
	mov qword ptr [r12+G_STATUS], G_WAITING
	lea rdi, [r12+G_LOCK]
	call rt_spin_unlock
	cmp qword ptr [r12+G_LOCKEDM], 0
	jne 2f
	call rt_set_idle
	jmp rt_schedule
2:
	call rt_set_idle
3:
	cmp qword ptr [r12+G_STATUS], G_WAITING
	jne 4f
	lea rdi, [r12+G_STATUS]
	mov esi, G_WAITING
	call rt_futex_wait
	jmp 3b
4:
	mov rax, r12
	jmp 9f
1:
	mov qword ptr [r12+G_TOKEN], 0
	lea rdi, [r12+G_LOCK]
	call rt_spin_unlock
	mov rdi, r12
	jmp rt_resume
9:
	mov r12, rax
	mov edi, IDLE
	call rt_m_running
	mov qword ptr [r12+G_STATUS], G_RUNNING
	mov qword ptr fs:[TLS_G], r12
	mov [rbx+M_CURG], r12
	mov rdi, r12
	jmp rt_resume

rt_resume:
	mov rsp, [rdi+G_SP]
	RESTORE_ALL
	ret

	.globl rt_poll
rt_poll:
	ENTER_RT
	SAVE_ALL
	mov byte ptr fs:[TLS_FLAG], 0
	cmp dword ptr [rip+rt_stop], 0
	je 1f
	call rt_stop_here
	jmp 2f
1:
	mov rbx, qword ptr fs:[TLS_M]
	cmp qword ptr [rbx+M_LOCKS], 0
	jne 2f
	mov r12, qword ptr fs:[TLS_G]
	cmp qword ptr [r12+G_LOCKEDM], 0
	jne 2f
	mov [r12+G_SP], rsp
	mov [r12+G_SCANLO], rsp
	mov rsp, [rbx+M_G0SP]
	mov qword ptr [r12+G_STATUS], G_RUNNABLE
	mov rdi, r12
	call rt_gq_put
	call rt_set_idle
	jmp rt_schedule
2:
	RESTORE_ALL
	ret

rt_schedule:
	mov rbx, qword ptr fs:[TLS_M]
1:
	call rt_timers_run
	xor eax, eax
	xchg [rbx+M_RUNNEXT], rax
	test rax, rax
	jnz 9f
	test qword ptr [rbx+M_TICK], 31
	jnz 7f
	cmp qword ptr [rip+rt_nfdwait], 0
	je 8f
	xor edi, edi
	call rt_netpoll
	test rax, rax
	jnz 1b
8:
	call rt_gq_get
	test rax, rax
	jnz 9f
7:
	mov rdi, rbx
	call rt_runq_get
	test rax, rax
	jnz 9f
	call rt_gq_get
	test rax, rax
	jnz 9f
	xor edi, edi
	call rt_netpoll
	test rax, rax
	jnz 1b
	call rt_steal
	test rax, rax
	jnz 9f
	cmp qword ptr [rbx+M_SPINNING], 0
	je 10f
	mov qword ptr [rbx+M_SPINNING], 0
	lock dec qword ptr [rip+rt_nspinning]
10:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_lock
	call rt_queues_empty
	test rax, rax
	jz 5f
	mov rax, [rip+rt_nidle]
	inc rax
	cmp rax, [rip+rt_nm]
	jne 2f
	cmp qword ptr [rip+rt_ntimers], 0
	jne 2f
	cmp qword ptr [rip+rt_nfdwait], 0
	jne 2f
	lea rdi, [rip+rt_msg_deadlock]
	jmp rt_panic
2:
	cmp qword ptr [rip+rt_poller], 0
	jne 3f
	cmp qword ptr [rip+rt_ntimers], 0
	jne 4f
	cmp qword ptr [rip+rt_nfdwait], 0
	je 3f
4:
	mov eax, 1
	xchg [rip+rt_poller], rax
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	call rt_timer_timeout
	mov rdi, rax
	call rt_netpoll
	mov qword ptr [rip+rt_poller], 0
	jmp 1b
3:
	mov rax, [rip+rt_idle]
	mov [rbx+M_NEXT], rax
	mov [rip+rt_idle], rbx
	inc qword ptr [rip+rt_nidle]
	mov qword ptr [rbx+M_PARK], 0
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
6:
	cmp qword ptr [rbx+M_PARK], 0
	jne 1b
	lea rdi, [rbx+M_PARK]
	xor esi, esi
	call rt_futex_wait
	jmp 6b
5:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	jmp 1b
9:
	mov r12, rax
	cmp qword ptr [rbx+M_SPINNING], 0
	je 11f
	mov qword ptr [rbx+M_SPINNING], 0
	lock dec qword ptr [rip+rt_nspinning]
	jnz 11f
	call rt_queues_empty
	test rax, rax
	jnz 11f
	call rt_wakep
11:
	mov edi, IDLE
	call rt_m_running
	mov qword ptr [r12+G_STATUS], G_RUNNING
	mov qword ptr fs:[TLS_G], r12
	mov [rbx+M_CURG], r12
	inc qword ptr [rbx+M_TICK]
	mov rdi, r12
	jmp rt_resume

rt_sysmon_start:
	mov eax, 1
	xchg [rip+rt_sysmon_on], rax
	test rax, rax
	jnz 1f
.if RT_DYNAMIC
	push rbx
	mov rbx, rsp
	and rsp, -16
	sub rsp, 16
	mov rdi, rsp
	xor esi, esi
	lea rdx, [rip+rt_sysmon]
	xor ecx, ecx
	call pthread_create
	mov rsp, rbx
	pop rbx
.else
	mov esi, 65536
	call rt_mmap_stack
	lea rsi, [rax+65536]
	mov edi, 0x50F00
	xor edx, edx
	xor r10d, r10d
	xor r8d, r8d
	mov eax, 56
	syscall
	test rax, rax
	jz rt_sysmon
.endif
1:
	ret

rt_sysmon:
	and rsp, -16
1:
	lea rdi, [rip+rt_sysmon_sleep]
	xor esi, esi
	mov eax, 35
	syscall
	xor ecx, ecx
2:
	cmp rcx, [rip+rt_nm]
	jae 4f
	lea rax, [rip+rt_allm]
	mov rdx, [rax+8*rcx]
	mov rax, [rdx+M_TICK]
	cmp qword ptr [rdx+M_STATE], RUNNING
	jne 5f
	cmp rax, [rdx+M_LASTTICK]
	jne 3f
	mov r8, [rdx+M_FLAG]
	mov byte ptr [r8], 1
	jmp 3f
5:
	cmp qword ptr [rdx+M_STATE], INC
	jne 3f
	xor edi, edi
	xchg [rdx+M_RUNNEXT], rdi
	test rdi, rdi
	jz 3f
	push rcx
	push rdx
	push rax
	call rt_gq_put
	call rt_wakep
	pop rax
	pop rdx
	pop rcx
3:
	mov [rdx+M_LASTTICK], rax
	inc rcx
	jmp 2b
4:
	cmp qword ptr [rip+rt_gq_n], 0
	je 1b
	cmp qword ptr [rip+rt_nidle], 0
	je 1b
	call rt_wakep
	jmp 1b

rt_now:
	sub rsp, 24
	mov edi, 1
	mov rsi, rsp
	mov eax, 228
	syscall
	imul rax, [rsp], 1000000000
	add rax, [rsp+8]
	add rsp, 24
	ret

rt_timer_insert:
	push rbx
	push r12
	push r13
	push r14
	push r15
	mov r12, rdi
	mov r13, rsi
	mov r14, rdx
	mov r15, rcx
	call rt_now
	imul r12, r12, 1000000
	add r12, rax
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_lock
	mov rcx, [rip+rt_ntimers]
	cmp rcx, MAXT
	jae 2f
	imul rax, rcx, 8*T_WORDS
	add rax, [rip+rt_timers]
	mov [rax], r12
	mov [rax+8], r13
	mov [rax+16], r14
	mov [rax+24], r15
	mov rbx, [rip+rt_timer_seq]
	inc rbx
	mov [rip+rt_timer_seq], rbx
	mov [rax+32], rbx
	inc rcx
	mov [rip+rt_ntimers], rcx
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_unlock
	call rt_poll_wake
	mov rax, rbx
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	ret
2:
	lea rdi, [rip+rt_msg_timers]
	jmp rt_panic

	.globl rt_sleep
rt_sleep:
	ENTER_RT
	push rdi
	call rt_poll_init
	pop rdi
	mov rsi, qword ptr fs:[TLS_G]
	xor edx, edx
	xor ecx, ecx
	call rt_timer_insert
	jmp rt_park_body

	.globl rt_timer_add
rt_timer_add:
	push rdi
	push rsi
	push rdx
	call rt_poll_init
	pop rcx
	pop rdx
	pop rdi
	mov rsi, qword ptr fs:[TLS_G]
	jmp rt_timer_insert

	.globl rt_timer_del
rt_timer_del:
	push rbx
	mov rbx, rdi
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_lock
	mov rcx, [rip+rt_ntimers]
	mov rdx, [rip+rt_timers]
	xor r8d, r8d
1:
	cmp r8, rcx
	jae 3f
	imul rax, r8, 8*T_WORDS
	cmp [rdx+rax+32], rbx
	je 2f
	inc r8
	jmp 1b
2:
	dec rcx
	mov [rip+rt_ntimers], rcx
	imul r9, rcx, 8*T_WORDS
	mov r10, [rdx+r9]
	mov [rdx+rax], r10
	mov r10, [rdx+r9+8]
	mov [rdx+rax+8], r10
	mov r10, [rdx+r9+16]
	mov [rdx+rax+16], r10
	mov r10, [rdx+r9+24]
	mov [rdx+rax+24], r10
	mov r10, [rdx+r9+32]
	mov [rdx+rax+32], r10
3:
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_unlock
	pop rbx
	ret

rt_timers_run:
	cmp qword ptr [rip+rt_ntimers], 0
	jne 1f
	xor eax, eax
	ret
1:
	push rbx
	push r12
	push r13
	push r14
	push r15
	call rt_now
	mov r14, rax
	xor r15d, r15d
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_lock
	xor r12d, r12d
2:
	cmp r12, [rip+rt_ntimers]
	jae 6f
	imul rbx, r12, 8*T_WORDS
	add rbx, [rip+rt_timers]
	cmp [rbx], r14
	jbe 3f
	inc r12
	jmp 2b
3:
	mov r13, [rbx+8]
	mov rdi, [rbx+16]
	test rdi, rdi
	jz 4f
	xor eax, eax
	mov ecx, 1
	lock cmpxchg [rdi], rcx
	jne 5f
	mov rax, [rbx+24]
	mov [rdi+8], rax
	mov qword ptr [rdi+16], 0
4:
	mov rdi, r13
	push rbx
	call rt_ready
	pop rbx
	inc r15
5:
	mov rcx, [rip+rt_ntimers]
	dec rcx
	mov [rip+rt_ntimers], rcx
	imul rax, rcx, 8*T_WORDS
	add rax, [rip+rt_timers]
	mov rdx, [rax]
	mov [rbx], rdx
	mov rdx, [rax+8]
	mov [rbx+8], rdx
	mov rdx, [rax+16]
	mov [rbx+16], rdx
	mov rdx, [rax+24]
	mov [rbx+24], rdx
	mov rdx, [rax+32]
	mov [rbx+32], rdx
	jmp 2b
6:
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_unlock
	mov rax, r15
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

rt_timer_timeout:
	push rbx
	push r12
	mov rax, -1
	cmp qword ptr [rip+rt_ntimers], 0
	je 4f
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_lock
	mov rbx, -1
	xor ecx, ecx
	mov rdx, [rip+rt_timers]
1:
	cmp rcx, [rip+rt_ntimers]
	jae 2f
	imul rax, rcx, 8*T_WORDS
	mov rax, [rdx+rax]
	cmp rax, rbx
	jae 3f
	mov rbx, rax
3:
	inc rcx
	jmp 1b
2:
	lea rdi, [rip+rt_timer_lock]
	call rt_spin_unlock
	mov rax, -1
	cmp rbx, -1
	je 4f
	call rt_now
	mov rcx, rbx
	sub rcx, rax
	xor eax, eax
	test rcx, rcx
	jle 4f
	lea rax, [rcx+999999]
	xor edx, edx
	mov ecx, 1000000
	div rcx
4:

	pop r12
	pop rbx
	ret

rt_poll_init:
	cmp qword ptr [rip+rt_epfd], 0
	je 1f
	ret
1:
	lea rdi, [rip+rt_poll_lock]
	call rt_spin_lock
	cmp qword ptr [rip+rt_epfd], 0
	jne 2f
	mov edi, 0x80000
	mov eax, 291
	syscall
	mov r8, rax
	xor edi, edi
	mov esi, 0x80800
	mov eax, 290
	syscall
	mov [rip+rt_evfd], rax
	sub rsp, 16
	mov dword ptr [rsp], 1
	mov qword ptr [rsp+4], 0
	mov rdi, r8
	mov esi, 1
	mov rdx, rax
	mov r10, rsp
	push r8
	mov eax, 233
	syscall
	pop r8
	add rsp, 16
	mov [rip+rt_epfd], r8
2:
	lea rdi, [rip+rt_poll_lock]
	jmp rt_spin_unlock

rt_poll_wake:
	mfence
	cmp qword ptr [rip+rt_poller], 0
	je 1f
	mov rdi, [rip+rt_evfd]
	test rdi, rdi
	jz 1f
	lea rsi, [rip+rt_one]
	mov edx, 8
	mov eax, 1
	syscall
1:
	ret

rt_netpoll:
	cmp qword ptr [rip+rt_epfd], 0
	jne 1f
	xor eax, eax
	ret
1:
	push rbx
	push r12
	push r13
	push r14
	mov r14, rdi
	sub rsp, 8*12*64/8
	mov r10, rdi
	mov rdi, [rip+rt_epfd]
	mov rsi, rsp
	mov edx, 64
	mov eax, 232
	syscall
	xor r13d, r13d
	test rax, rax
	jle 4f
	mov r12, rax
	xor ebx, ebx
2:
	cmp rbx, r12
	jae 4f
	imul rcx, rbx, 12
	mov rdi, [rsp+rcx+4]
	inc rbx
	test rdi, rdi
	jnz 3f
	test r14, r14
	jz 2b
	mov rdi, [rip+rt_evfd]
	lea rsi, [rip+rt_scratch]
	mov edx, 8
	xor eax, eax
	syscall
	jmp 2b
3:
	lock dec qword ptr [rip+rt_nfdwait]
	call rt_ready
	inc r13
	jmp 2b
4:
	mov rax, r13
	add rsp, 8*12*64/8
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_wait_fd
rt_wait_fd:
	ENTER_RT
	push rdi
	push rsi
	call rt_poll_init
	pop rsi
	pop rdi
	lock inc qword ptr [rip+rt_nfdwait]
	sub rsp, 16
	or esi, 0x40000000
	mov [rsp], esi
	mov rax, qword ptr fs:[TLS_G]
	mov [rsp+4], rax
	mov rdx, rdi
	push rdx
	mov rdi, [rip+rt_epfd]
	mov esi, 3
	lea r10, [rsp+8]
	mov eax, 233
	syscall
	pop rdx
	cmp rax, -2
	jne 1f
	mov rdi, [rip+rt_epfd]
	mov esi, 1
	mov r10, rsp
	mov eax, 233
	syscall
1:
	add rsp, 16
	jmp rt_park_body

	.section .rodata
	.p2align 3
rt_one:
	.quad 1
	.text
