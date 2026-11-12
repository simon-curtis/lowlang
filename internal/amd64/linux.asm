	.section .tbss,"awT",@nobits
	.p2align 3
	.zero 24

	.section .rodata
rt_sysmon_sleep:
	.quad 0
	.quad 10000000

	.bss
	.p2align 3
rt_epfd:
	.zero 8
rt_evfd:
	.zero 8
rt_poll_lock:
	.zero 8
rt_scratch:
	.zero 128

	.text
rt_write:
	push r12
	push r13
	push r14
	mov r12d, edi
	mov r13, rsi
	mov r14, rdx
1:
	test r14, r14
	jz 2f
	mov edi, r12d
	mov rsi, r13
	mov rdx, r14
	mov eax, 1
	syscall
	cmp rax, -4
	je 1b
	test rax, rax
	js 2f
	add r13, rax
	sub r14, rax
	jmp 1b
2:
	pop r14
	pop r13
	pop r12
	ret

	.globl rt_print_str
rt_print_str:
	push rbx
	push r12
	sub rsp, 40
	mov rbx, rdi
	mov r12, rsi
	mov [rsp], rdi
	mov [rsp+8], rsi
	lea rax, [rip+rt_nl]
	mov [rsp+16], rax
	mov qword ptr [rsp+24], 1
1:
	mov edi, 1
	mov rsi, rsp
	mov edx, 2
	mov eax, 20
	syscall
	cmp rax, -4
	je 1b
	test rax, rax
	js 3f
	cmp rax, r12
	jae 2f
	lea rsi, [rbx+rax]
	mov rdx, r12
	sub rdx, rax
	mov edi, 1
	call rt_write
	xor eax, eax
	mov r12, rax
2:
	cmp rax, r12
	ja 3f
	mov edi, 1
	lea rsi, [rip+rt_nl]
	mov edx, 1
	call rt_write
3:
	add rsp, 40
	pop r12
	pop rbx
	ret

rt_exit:
	mov eax, 231
	syscall

rt_env_byte:
	mov rsi, [rip+rt_env_values]
	test rsi, rsi
	jz 7f
1:
	mov r8, [rsi]
	test r8, r8
	jz 7f
	xor ecx, ecx
2:
	movzx eax, byte ptr [rdi+rcx]
	test al, al
	jz 3f
	cmp al, [r8+rcx]
	jne 6f
	inc rcx
	jmp 2b
3:
	cmp byte ptr [r8+rcx], 61
	jne 6f
	movzx eax, byte ptr [r8+rcx+1]
	ret
6:
	add rsi, 8
	jmp 1b
7:
	xor eax, eax
	ret

rt_mmap:
	xor edi, edi
	mov edx, 3
	mov r10d, 0x4022
	mov r8, -1
	xor r9d, r9d
	mov eax, 9
	syscall
	cmp rax, -4096
	ja rt_panic_oom
	ret

rt_mmap_sparse:
	push rsi
	call rt_mmap
	pop rsi
	push rax
	mov rdi, rax
	mov edx, 15
	mov eax, 28
	syscall
	pop rax
	ret

rt_heap_release:
	push rbx
	push r12
	push r13
	mov ebx, BIN_RELEASE
1:
	mov rdi, rbx
	lea rsi, [rip+rt_bin_bits]
	call rt_bits_next
	cmp rax, -1
	je 9f
	mov rbx, rax
2:
	lea rcx, [rip+rt_bins]
	mov r12, [rcx+8*rbx]
	mov rax, [r12+8]
	mov [rcx+8*rbx], rax
	test rax, rax
	jnz 3f
	lea rcx, [rip+rt_bin_bits]
	btr qword ptr [rcx], rbx
3:
	mov rax, [r12]
	shr rax, 3
	lea r13, [r12+8*rax]
	and r13, -4096
	lea rdi, [r12+16+4095]
	and rdi, -4096
	mov rsi, r13
	sub rsi, rdi
	mov edx, 4
	mov eax, 28
	syscall
	mov rcx, [rip+rt_heap_start]
	sub rdi, rcx
	shr rdi, PAGE_SHIFT
	sub r13, rcx
	shr r13, PAGE_SHIFT
	dec r13
	mov rsi, r13
	test rax, rax
	jnz 4f
	call rt_page_clear
4:
	lea rdx, [rbx+BIN_CLEAN]
	lea rcx, [rip+rt_bins]
	mov rax, [rcx+8*rdx]
	mov [r12+8], rax
	mov [rcx+8*rdx], r12
	lea rcx, [rip+rt_bin_bits]
	bts qword ptr [rcx], rdx
	lea rcx, [rip+rt_bins]
	cmp qword ptr [rcx+8*rbx], 0
	jne 2b
	jmp 1b
9:
	pop r13
	pop r12
	pop rbx
	ret

rt_yield:
	mov eax, 24
	syscall
	ret

	.globl rt_sysblock
rt_sysblock:
	push rbx
	push r12
	push r13
	push r14
	push r15
	push r9
	push r8
	push r10
	push rdx
	push rsi
	push rdi
	push rax
	TLS_GET rcx, TLS_G
	mov r11, [rsp+96]
	mov [rcx+G_PC], r11
	lea r11, [rsp+104]
	mov [rcx+G_USERSP], r11
	mov [rcx+G_FP], rbp
	mov [rcx+G_SCANLO], rsp
	TLS_GET r11, TLS_M
	mov ecx, INC
	xchg [r11+M_STATE], rcx
	mov rax, [rsp]
	mov rdi, [rsp+8]
	mov rsi, [rsp+16]
	mov rdx, [rsp+24]
	mov r10, [rsp+32]
	mov r8, [rsp+40]
	mov r9, [rsp+48]
	syscall
	call rt_exitc
	add rsp, 56
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
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

rt_os_stack:
	mov rsi, G_MAP
	call rt_mmap_stack
	push rax
	mov rdi, rax
	mov esi, GUARD
	xor edx, edx
	mov eax, 10
	syscall
	pop rax
	ret

.if RT_DYNAMIC
.else
rt_os_g0_stack:
	mov rsi, G0_STACK
	call rt_mmap_stack
	mov rdi, rax
	mov esi, G0_STACK
	mov edx, 15
	mov eax, 28
	syscall
	lea rax, [rdi+G0_STACK]
	mov [rbx+M_G0TOP], rax
	mov [rbx+M_G0SP], rax
	ret
.endif

rt_os_m_start:
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
	ret

.if RT_DYNAMIC
rt_mstart_c:
	mov rbx, rdi
	call rt_os_tls
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

rt_os_init:
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
	ret

rt_os_tls:
.if RT_DYNAMIC
	mov qword ptr fs:[TLS_M], rbx
	mov rax, qword ptr fs:[0]
	add rax, TLS_FLAG
	mov [rbx+M_FLAG], rax
.else
	call rt_tcb_init
	mov rsi, rax
	mov edi, 0x1002
	mov eax, 158
	syscall
.endif
	ret

rt_os_signals:
	call rt_altstack
	jmp rt_fault_init

rt_fault_init:
	mov edi, 11
	lea rsi, [rip+rt_fault]
	mov edx, 0x0C000004
	call rt_sigaction
	mov edi, 7
	lea rsi, [rip+rt_fault]
	mov edx, 0x0C000004
	jmp rt_sigaction

rt_sigaction:
	sub rsp, 32
	mov [rsp], rsi
	mov [rsp+8], rdx
	lea rax, [rip+rt_restorer]
	mov [rsp+16], rax
	mov qword ptr [rsp+24], 0
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

rt_os_sysmon_start:
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
	ret

rt_os_nap:
	lea rdi, [rip+rt_sysmon_sleep]
	xor esi, esi
	mov eax, 35
	syscall
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

	.data
rt_vdso_clock:
	.quad 0

	.section .rodata
rt_vdso_clock_name:
	.asciz "__vdso_clock_gettime"

	.text
rt_auxv:
	test rdi, rdi
	jz 3f
1:
	mov rax, [rdi]
	add rdi, 8
	test rax, rax
	jnz 1b
2:
	mov rax, [rdi]
	test rax, rax
	jz 3f
	cmp rax, rsi
	je 4f
	add rdi, 16
	jmp 2b
3:
	xor eax, eax
	ret
4:
	mov rax, [rdi+8]
	ret

rt_seed_init:
	mov esi, 25
	call rt_auxv
	mov rsi, rax
	test rsi, rsi
	jz 5f
	mov rax, [rsi]
	mov rdx, [rsi+8]
	jmp 8f
5:
	lea rdi, [rip+rt_hash_seed]
	mov esi, 16
	xor edx, edx
	mov eax, 318
	syscall
	cmp rax, 16
	je 9f
	call rt_seed_fallback
8:
	mov [rip+rt_hash_seed], rax
	mov [rip+rt_hash_seed+8], rdx
9:
	or qword ptr [rip+rt_hash_seed+8], 1
	ret

rt_vdso_init:
	push rbx
	push r12
	push r13
	push r14
	push r15
	mov esi, 33
	call rt_auxv
	mov r8, rax
	test r8, r8
	jz 9f
	cmp dword ptr [r8], 0x464c457f
	jne 9f
	cmp word ptr [r8+4], 0x0102
	jne 9f
	mov rsi, [r8+32]
	add rsi, r8
	movzx ecx, word ptr [r8+56]
	movzx edx, word ptr [r8+54]
	xor r9d, r9d
	xor r10d, r10d
	xor r11d, r11d
4:
	test ecx, ecx
	jz 6f
	mov eax, [rsi]
	cmp eax, 1
	jne 5f
	test r11d, r11d
	jnz 51f
	mov r9, r8
	add r9, [rsi+8]
	sub r9, [rsi+16]
	mov r11d, 1
	jmp 51f
5:
	cmp eax, 2
	jne 51f
	mov r10, [rsi+8]
	add r10, r8
51:
	add rsi, rdx
	dec ecx
	jmp 4b
6:
	test r11d, r11d
	jz 9f
	test r10, r10
	jz 9f
	xor r12d, r12d
	xor r13d, r13d
	xor r14d, r14d
	xor r15d, r15d
7:
	mov rax, [r10]
	test rax, rax
	jz 8f
	mov rcx, [r10+8]
	add rcx, r9
	add r10, 16
	cmp rax, 5
	jne 71f
	mov r12, rcx
	jmp 7b
71:
	cmp rax, 6
	jne 72f
	mov r13, rcx
	jmp 7b
72:
	cmp rax, 4
	jne 73f
	mov r14, rcx
	jmp 7b
73:
	cmp rax, 0x6ffffef5
	jne 7b
	mov r15, rcx
	jmp 7b
8:
	test r12, r12
	jz 9f
	test r13, r13
	jz 9f
	test r14, r14
	jz 81f
	mov ebx, [r14+4]
	jmp 85f
81:
	test r15, r15
	jz 9f
	mov ecx, [r15]
	mov esi, [r15+4]
	mov eax, [r15+8]
	lea rdx, [r15+16+8*rax]
	xor ebx, ebx
	xor eax, eax
82:
	cmp eax, ecx
	jae 83f
	cmp ebx, [rdx+4*rax]
	cmovb ebx, [rdx+4*rax]
	inc eax
	jmp 82b
83:
	lea rdx, [rdx+4*rcx]
	cmp ebx, esi
	jae 84f
	mov ebx, esi
	jmp 85f
84:
	mov eax, ebx
	sub eax, esi
	test dword ptr [rdx+4*rax], 1
	jnz 841f
	inc ebx
	jmp 84b
841:
	inc ebx
85:
	xor ecx, ecx
86:
	cmp ecx, ebx
	jae 9f
	lea rax, [rcx+2*rcx]
	lea rax, [r13+8*rax]
	inc ecx
	movzx edx, byte ptr [rax+4]
	mov esi, edx
	and esi, 15
	cmp esi, 2
	jne 86b
	shr edx, 4
	dec edx
	cmp edx, 1
	ja 86b
	cmp word ptr [rax+6], 0
	je 86b
	mov esi, [rax]
	add rsi, r12
	lea rdi, [rip+rt_vdso_clock_name]
87:
	movzx edx, byte ptr [rdi]
	cmp dl, [rsi]
	jne 86b
	inc rdi
	inc rsi
	test dl, dl
	jnz 87b
	mov rax, [rax+8]
	add rax, r9
	mov [rip+rt_vdso_clock], rax
9:
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_clock_nanos
rt_clock_nanos:
	push rbp
	mov rbp, rsp
	sub rsp, 16
	and rsp, -16
	mov rsi, rsp
	mov rax, [rip+rt_vdso_clock]
	test rax, rax
	jz 1f
	call rax
	jmp 2f
1:
	mov eax, 228
	syscall
2:
	test rax, rax
	jnz 3f
	imul rax, [rsp], 1000000000
	add rax, [rsp+8]
	mov rsp, rbp
	pop rbp
	ret
3:
	xor eax, eax
	mov rsp, rbp
	pop rbp
	ret
