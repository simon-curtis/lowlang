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
rt_poller:
	.zero 8
rt_nfdwait:
	.zero 8
rt_nblocked:
	.zero 8

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
	call rt_yield
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
	TLS_GET rbx, TLS_M
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
	TLS_GET rax, TLS_G
	mov [rax+G_SCANLO], rsp
	TLS_GET rbx, TLS_M
	mov eax, STOPPED
	xchg [rbx+M_STATE], rax
	mov edi, STOPPED
	call rt_m_running
	RESTORE_ALL
	ret

rt_set_idle:
	TLS_GET rcx, TLS_M
	mov qword ptr [rcx+M_CURG], 0
	TLS_SET TLS_G, 0
	mov qword ptr [rcx+M_STATE], IDLE
	ret

	.globl rt_lock
rt_lock:
	ENTER_RT
rt_lock_body:
	xor eax, eax
	mov ecx, 1
	lock cmpxchg dword ptr [rdi], ecx
	jz 9f
	xor r8d, r8d
1:
	cmp dword ptr [rip+rt_stop], 0
	jne 6f
	mov ecx, r8d
	mov r9d, LOCK_SPIN_TICKS
	shl r9d, cl
	rdtsc
	shl rdx, 32
	or rax, rdx
	add r9, rax
2:
	pause
	rdtsc
	shl rdx, 32
	or rax, rdx
	cmp rax, r9
	jb 2b
	mov eax, [rdi]
	test eax, eax
	jnz 3f
	mov ecx, 1
	lock cmpxchg dword ptr [rdi], ecx
	jz 9f
3:
	inc r8d
	cmp r8d, LOCK_SPIN_ROUNDS
	jb 1b
	mov r8d, LOCK_YIELDS
4:
	push rdi
	call rt_yield
	pop rdi
	xor eax, eax
	mov ecx, 1
	lock cmpxchg dword ptr [rdi], ecx
	jz 9f
	dec r8d
	jnz 4b
5:
	mov eax, 2
	xchg dword ptr [rdi], eax
	test eax, eax
	jz 9f
	push rdi
	call rt_lock_sleep
	pop rdi
	jmp 5b
6:
	push rdi
	call rt_stop_here
	pop rdi
	jmp rt_lock_body
9:
	TLS_GET rax, TLS_M
	inc qword ptr [rax+M_LOCKS]
	ret

rt_lock_sleep:
	SAVE_ALL
	TLS_GET rax, TLS_G
	mov [rax+G_SCANLO], rsp
	TLS_GET rbx, TLS_M
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
	TLS_GET rax, TLS_M
	dec qword ptr [rax+M_LOCKS]
	xor eax, eax
	xchg dword ptr [rdi], eax
	cmp eax, 2
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
	TLS_GET rax, TLS_G
	ret

	.globl rt_context
rt_context:
	TLS_GET rax, TLS_G
	mov rax, [rax+G_CTX]
	ret

	.globl rt_set_context
rt_set_context:
	TLS_GET rax, TLS_G
	mov [rax+G_CTX], rdi
	ret

	.globl rt_has_context
rt_has_context:
	TLS_GET rax, TLS_G
	cmp qword ptr [rax+G_CTX], 0
	setne al
	movzx eax, al
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
	call rt_os_stack
	mov rbx, rax
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
	cmp qword ptr [rip+rt_gq_n], 0
	je 3f
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
3:
	xor eax, eax
	ret

rt_gq_grab:
	cmp qword ptr [rip+rt_gq_n], 0
	je 8f
	push rbx
	push r13
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_lock
	mov rbx, [rip+rt_gq_head]
	test rbx, rbx
	jz 6f
	mov rax, [rip+rt_gq_n]
	xor edx, edx
	div qword ptr [rip+rt_ncpu]
	inc rax
	cmp rax, GQ_BATCH
	jbe 1f
	mov eax, GQ_BATCH
1:
	mov rcx, rbx
	mov r8d, 1
2:
	cmp r8, rax
	jae 3f
	mov rdx, [rcx+G_NEXT]
	test rdx, rdx
	jz 3f
	mov rcx, rdx
	inc r8
	jmp 2b
3:
	mov rdx, [rcx+G_NEXT]
	mov [rip+rt_gq_head], rdx
	test rdx, rdx
	jnz 4f
	mov qword ptr [rip+rt_gq_tail], 0
4:
	mov qword ptr [rcx+G_NEXT], 0
	sub [rip+rt_gq_n], r8
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	mov r13, [rbx+G_NEXT]
	mov qword ptr [rbx+G_NEXT], 0
5:
	test r13, r13
	jz 7f
	mov rdi, r13
	mov r13, [r13+G_NEXT]
	call rt_runq_put
	jmp 5b
6:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
7:
	mov rax, rbx
	pop r13
	pop rbx
	ret
8:
	xor eax, eax
	ret

rt_runq_put:
	push rbx
	push r12
	mov r12, rdi
	TLS_GET rbx, TLS_M
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
	mov rcx, [rbx+M_QHEAD]
	cmp rcx, [rbx+M_QTAIL]
	jne 2f
	xor eax, eax
	pop rbx
	ret
2:
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

rt_runq_steal:
	push rbx
	push r12
	push r13
	push r14
	mov rbx, rdi
	TLS_GET r12, TLS_M
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_lock
	mov rcx, [rbx+M_QHEAD]
	mov rdx, [rbx+M_QTAIL]
	sub rdx, rcx
	jz 4f
	mov rax, rdx
	shr rax, 1
	sub rdx, rax
	mov r8, rcx
	and r8d, 255
	mov r13, [rbx+M_QUEUE+8*r8]
	mov r9, [r12+M_QTAIL]
	lea r14, [rdx-1]
	xor esi, esi
	lea r11, [rcx+1]
1:
	cmp rsi, r14
	jae 2f
	mov r8, r11
	and r8d, 255
	mov rax, [rbx+M_QUEUE+8*r8]
	lea r8, [r9+rsi]
	and r8d, 255
	mov [r12+M_QUEUE+8*r8], rax
	inc r11
	inc rsi
	jmp 1b
2:
	add rcx, rdx
	mov [rbx+M_QHEAD], rcx
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_unlock
	test r14, r14
	jz 3f
	lea rdi, [r12+M_QLOCK]
	call rt_spin_lock
	add [r12+M_QTAIL], r14
	lea rdi, [r12+M_QLOCK]
	call rt_spin_unlock
3:
	mov rax, r13
	pop r14
	pop r13
	pop r12
	pop rbx
	ret
4:
	lea rdi, [rbx+M_QLOCK]
	call rt_spin_unlock
	xor eax, eax
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

rt_steal_runnext:
	mov rax, [rdi+M_STATE]
	and eax, 7
	cmp eax, INC
	je 2f
	cmp eax, STOPPED
	je 2f
	cmp eax, RUNNING
	jne 3f
	mov r8, [rdi+M_RNSEQ]
	rdtsc
	shl rdx, 32
	or rax, rdx
	lea rcx, [rax+STEAL_DELAY]
1:
	pause
	cmp [rdi+M_RNSEQ], r8
	jne 3f
	rdtsc
	shl rdx, 32
	or rax, rdx
	cmp rax, rcx
	jb 1b
2:
	xor eax, eax
	xchg [rdi+M_RUNNEXT], rax
	ret
3:
	xor eax, eax
	ret

rt_steal:
	push rbx
	push r12
	push r13
	push r14
	xor r13d, r13d
1:
	rdtsc
	mov eax, eax
	xor edx, edx
	div qword ptr [rip+rt_nm]
	mov r12, rdx
	xor r14d, r14d
2:
	cmp r14, [rip+rt_nm]
	jae 7f
	lea rax, [r12+r14]
	cmp rax, [rip+rt_nm]
	jb 3f
	sub rax, [rip+rt_nm]
3:
	lea rcx, [rip+rt_allm]
	mov rdi, [rcx+8*rax]
	inc r14
	TLS_GET rax, TLS_M
	cmp rdi, rax
	je 2b
	mov rax, [rdi+M_QHEAD]
	cmp rax, [rdi+M_QTAIL]
	jne 4f
	cmp r13, STEAL_ROUNDS-1
	jb 2b
	cmp qword ptr [rdi+M_RUNNEXT], 0
	je 2b
	call rt_steal_runnext
	test rax, rax
	jnz 9f
	jmp 2b
4:
	call rt_runq_steal
	test rax, rax
	jnz 9f
	jmp 2b
7:
	inc r13
	cmp r13, STEAL_ROUNDS
	jb 1b
	xor eax, eax
9:
	pop r14
	pop r13
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
	cmp qword ptr [rdx+M_RUNNEXT], 0
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
	TLS_GET rax, TLS_M
	mov rdi, rbx
	inc qword ptr [rax+M_RNSEQ]
	xchg [rax+M_RUNNEXT], rdi
	test rdi, rdi
	jz 3f
	call rt_runq_put
3:
	call rt_wakep
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
	jne 2f
	cmp qword ptr [rip+rt_nidle], 0
	jne 1f
	mov rax, [rip+rt_nm]
	sub rax, [rip+rt_nblocked]
	cmp rax, [rip+rt_ncpu]
	jae 2f
1:
	rdtsc
	shl rdx, 32
	or rax, rdx
	TLS_GET rcx, TLS_M
	mov rdx, rax
	sub rdx, [rcx+M_LASTWAKE]
	mov r8, [rcx+M_TICK]
	cmp rdx, WAKE_INTERVAL
	jae 3f
	cmp r8, [rcx+M_WAKETICK]
	jne 2f
3:
	mov [rcx+M_LASTWAKE], rax
	mov [rcx+M_WAKETICK], r8
	jmp rt_wakep_locked
2:
	ret

rt_wakep_full:
	cmp qword ptr [rip+rt_nspinning], 0
	je rt_wakep_locked
	ret

rt_wakep_locked:
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
	mov qword ptr [rbx+M_SPINNING], 1
	lock inc qword ptr [rip+rt_nspinning]
	call rt_os_m_prepare
	mov rcx, [rip+rt_nm]
	lea rax, [rip+rt_allm]
	mov [rax+8*rcx], rbx
	inc rcx
	mov [rip+rt_nm], rcx
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	call rt_os_m_start
	pop r12
	pop rbx
	ret

	.globl rt_sched_init
rt_sched_init:
	push rbx
	call rt_os_init
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
	call rt_os_tls
	lea rax, [rsp+16]
	and rax, -16
	mov [rbx+M_G0TOP], rax
	mov [rbx+M_G0SP], rax
	call rt_os_signals
	pop rbx
	ret

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
	TLS_GET rax, TLS_G
	test rax, rax
	jz 1f
	mov rax, [rax+G_CTX]
1:
	mov [rbx+G_CTX], rax
	mov qword ptr [rbx+G_STATUS], G_RUNNABLE
	mov rdi, rbx
	TLS_GET rax, TLS_M
	mov rcx, [rax+M_QTAIL]
	sub rcx, [rax+M_QHEAD]
	cmp rcx, SPAWN_LOCAL
	jae 2f
	call rt_runq_put
	jmp 3f
2:
	call rt_gq_put
3:
	call rt_wakep_full
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
	TLS_GET rdi, TLS_G
	TLS_GET rbx, TLS_M
	TEB_ENTER_OS rdi, rbx
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
	TLS_GET r12, TLS_G
	mov [r12+G_SP], rsp
	mov [r12+G_SCANLO], rsp
	TLS_GET rbx, TLS_M
	TEB_ENTER_OS r12, rbx
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
	TLS_SET TLS_G, r12
	mov [rbx+M_CURG], r12
	mov rdi, r12
	jmp rt_resume

rt_resume:
	TEB_ENTER_G rdi, rbx
	mov rsp, [rdi+G_SP]
	RESTORE_ALL
	ret

	.globl rt_poll
rt_poll:
	ENTER_RT
	sub rsp, 120
	movq [rsp], xmm2
	movq [rsp+8], xmm3
	movq [rsp+16], xmm4
	movq [rsp+24], xmm5
	movq [rsp+32], xmm6
	movq [rsp+40], xmm7
	movq [rsp+48], xmm8
	movq [rsp+56], xmm9
	movq [rsp+64], xmm10
	movq [rsp+72], xmm11
	movq [rsp+80], xmm12
	movq [rsp+88], xmm13
	movq [rsp+96], xmm14
	movq [rsp+104], xmm15
	call rt_poll_run
	movq xmm2, [rsp]
	movq xmm3, [rsp+8]
	movq xmm4, [rsp+16]
	movq xmm5, [rsp+24]
	movq xmm6, [rsp+32]
	movq xmm7, [rsp+40]
	movq xmm8, [rsp+48]
	movq xmm9, [rsp+56]
	movq xmm10, [rsp+64]
	movq xmm11, [rsp+72]
	movq xmm12, [rsp+80]
	movq xmm13, [rsp+88]
	movq xmm14, [rsp+96]
	movq xmm15, [rsp+104]
	add rsp, 120
	ret

rt_poll_run:
	SAVE_ALL
	TLS_CLEAR_FLAG
	cmp dword ptr [rip+rt_stop], 0
	je 1f
	call rt_stop_here
	jmp 2f
1:
	TLS_GET rbx, TLS_M
	cmp qword ptr [rbx+M_LOCKS], 0
	jne 2f
	TLS_GET r12, TLS_G
	cmp qword ptr [r12+G_LOCKEDM], 0
	jne 2f
	mov [r12+G_SP], rsp
	mov [r12+G_SCANLO], rsp
	TEB_ENTER_OS r12, rbx
	mov rsp, [rbx+M_G0SP]
	mov qword ptr [r12+G_STATUS], G_RUNNABLE
	mov rdi, r12
	call rt_gq_put
	call rt_wakep
	call rt_set_idle
	jmp rt_schedule
2:
	RESTORE_ALL
	ret

rt_schedule:
	TLS_GET rbx, TLS_M
1:
	call rt_timers_run
	inc qword ptr [rbx+M_FAIR]
	cmp qword ptr [rbx+M_FAIR], FAIR_INTERVAL
	jb 3f
	mov qword ptr [rbx+M_FAIR], 0
	cmp qword ptr [rip+rt_nfdwait], 0
	je 2f
	xor edi, edi
	call rt_netpoll
	test rax, rax
	jnz 1b
2:
	call rt_gq_get
	test rax, rax
	jnz 9f
3:
	cmp qword ptr [rbx+M_RUNNEXT], 0
	je 15f
	xor eax, eax
	xchg [rbx+M_RUNNEXT], rax
	test rax, rax
	jz 15f
	test qword ptr [rbx+M_TICK], 31
	jnz 9f
	mov rdi, rax
	call rt_runq_put
15:
	mov rdi, rbx
	call rt_runq_get
	test rax, rax
	jnz 9f
	call rt_gq_grab
	test rax, rax
	jnz 9f
	cmp qword ptr [rip+rt_nfdwait], 0
	je 4f
	xor edi, edi
	call rt_netpoll
	test rax, rax
	jnz 1b
4:
	cmp qword ptr [rbx+M_SPINNING], 0
	jne 5f
	mov rax, [rip+rt_nspinning]
	add rax, rax
	mov rcx, [rip+rt_nm]
	sub rcx, [rip+rt_nidle]
	cmp rax, rcx
	jae 10f
	mov qword ptr [rbx+M_SPINNING], 1
	lock inc qword ptr [rip+rt_nspinning]
5:
	call rt_steal
	test rax, rax
	jnz 9f
10:
	cmp qword ptr [rbx+M_SPINNING], 0
	je 13f
	mov qword ptr [rbx+M_SPINNING], 0
	lock dec qword ptr [rip+rt_nspinning]
13:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_lock
	call rt_queues_empty
	test rax, rax
	jz 6f
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
7:
	cmp qword ptr [rbx+M_PARK], 0
	jne 1b
	lea rdi, [rbx+M_PARK]
	xor esi, esi
	call rt_futex_wait
	jmp 7b
6:
	lea rdi, [rip+rt_sched_lock]
	call rt_spin_unlock
	mov qword ptr [rbx+M_SPINNING], 1
	lock inc qword ptr [rip+rt_nspinning]
	jmp 1b
9:
	mov r12, rax
	inc qword ptr [rbx+M_TICK]
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
	TLS_SET TLS_G, r12
	mov [rbx+M_CURG], r12
	mov rdi, r12
	jmp rt_resume

rt_sysmon_start:
	mov eax, 1
	xchg [rip+rt_sysmon_on], rax
	test rax, rax
	jz rt_os_sysmon_start
	ret

rt_sysmon:
	and rsp, -16
1:
	call rt_os_nap
	xor ecx, ecx
	xor r12d, r12d
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
	cmp rax, [rdx+M_LASTTICK]
	jne 6f
	inc r12
6:
	push rcx
	push rdx
	push rax
	xor edi, edi
	xchg [rdx+M_RUNNEXT], rdi
	test rdi, rdi
	jz 7f
	call rt_gq_put
	jmp 8f
7:
	mov rax, [rdx+M_QHEAD]
	cmp rax, [rdx+M_QTAIL]
	je 9f
8:
	call rt_wakep_full
9:
	pop rax
	pop rdx
	pop rcx
3:
	mov [rdx+M_LASTTICK], rax
	inc rcx
	jmp 2b
4:
	mov [rip+rt_nblocked], r12
	cmp qword ptr [rip+rt_gq_n], 0
	je 1b
	cmp qword ptr [rip+rt_nidle], 0
	je 1b
	call rt_wakep_full
	jmp 1b

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
	TLS_GET rsi, TLS_G
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
	TLS_GET rsi, TLS_G
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
