	.intel_syntax noprefix

	.section .rodata
	.p2align 5
	.globl rt_vsign8
rt_vsign8:
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.byte 0x80
	.globl rt_vsign16
rt_vsign16:
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.short 0x8000
	.globl rt_vsign32
rt_vsign32:
	.long 0x80000000
	.long 0x80000000
	.long 0x80000000
	.long 0x80000000
	.long 0x80000000
	.long 0x80000000
	.long 0x80000000
	.long 0x80000000
	.globl rt_vsign64
rt_vsign64:
	.quad 0x8000000000000000
	.quad 0x8000000000000000
	.quad 0x8000000000000000
	.quad 0x8000000000000000
rt_simd_env:
	.asciz "LOW_SIMD=sse2"
rt_true:
	.ascii "true\n"
rt_false:
	.ascii "false\n"
rt_nl:
	.ascii "\n"
rt_panic_prefix:
	.ascii "panic: "
rt_error_prefix:
	.ascii "error: "
rt_msg_divzero:
	.quad 22
	.ascii "integer divide by zero"
rt_msg_pins:
	.quad 13
	.ascii "too many pins"
rt_msg_index:
	.quad 18
	.ascii "index out of range"
rt_msg_oom:
	.quad 13
	.ascii "out of memory"

	.bss
	.p2align 3
rt_heap_lock:
	.zero 8
rt_heap_start:
	.zero 8
rt_heap_end:
	.zero 8
rt_heap_bump:
	.zero 8
rt_bitmap:
	.zero 8
rt_summary1:
	.zero 8
rt_summary2:
	.zero 8
rt_mark_base:
	.zero 8
rt_alloc_since:
	.zero 8
rt_gc_threshold:
	.zero 8
rt_free_large:
	.zero 8
rt_pin_count:
	.zero 8
rt_pins:
	.zero 524288
rt_free_small:
	.zero 512
rt_arg_count:
	.zero 8
rt_arg_values:
	.zero 8
rt_env_values:
	.zero 8

	.text

	.globl rt_print_bool
rt_print_bool:
	lea rsi, [rip+rt_true]
	mov edx, 5
	lea rax, [rip+rt_false]
	test rdi, rdi
	cmovz rsi, rax
	mov eax, 6
	cmovz edx, eax
	mov edi, 1
	jmp rt_write

	.globl rt_print_i64
rt_print_i64:
	sub rsp, 40
	lea rsi, [rsp+32]
	mov byte ptr [rsi], 10
	mov rax, rdi
	mov r8, rdi
	test rax, rax
	jns 1f
	neg rax
1:
	mov ecx, 10
2:
	xor edx, edx
	div rcx
	add dl, 48
	dec rsi
	mov [rsi], dl
	test rax, rax
	jnz 2b
	test r8, r8
	jns 3f
	dec rsi
	mov byte ptr [rsi], 45
3:
	lea rdx, [rsp+33]
	sub rdx, rsi
	mov edi, 1
	call rt_write
	add rsp, 40
	ret

	.globl rt_str_eq
rt_str_eq:
	xor eax, eax
	cmp rsi, rcx
	jne 1f
	mov rsi, rdx
	test rcx, rcx
	repe cmpsb
	sete al
1:
	ret

	.globl rt_memmove
rt_memmove:
	mov rcx, rdx
	cmp rdi, rsi
	jbe 1f
	lea rax, [rsi+rdx]
	cmp rdi, rax
	jae 1f
	lea rsi, [rsi+rdx-1]
	lea rdi, [rdi+rdx-1]
	std
	rep movsb
	cld
	ret
1:
	rep movsb
	ret

	.globl rt_grow
rt_grow:
	ENTER_RT
	push rdi
	push rsi
	push rdx
	push rcx
	lea rdi, [rdx+7]
	shr rdi, 3
	mov rsi, rcx
	cmp rdi, 1
	adc rdi, 0
	cmp rdi, ALLOC_SMALL
	ja 1f
	call rt_alloc_small
	jmp 2f
1:
	call rt_heap_acquire
	call rt_alloc_body
	mov qword ptr [rip+rt_heap_lock], 0
2:
	pop rcx
	pop rdx
	pop rcx
	pop rsi
	mov rdi, rax
	rep movsb
	ret

	.globl rt_pin
rt_pin:
	ENTER_RT
	call rt_heap_acquire
	mov rax, [rip+rt_pin_count]
	cmp rax, 65536
	jae 1f
	lea rcx, [rip+rt_pins]
	mov [rcx+8*rax], rdi
	inc rax
	mov [rip+rt_pin_count], rax
	mov qword ptr [rip+rt_heap_lock], 0
	ret
1:
	lea rdi, [rip+rt_msg_pins]
	jmp rt_panic

	.globl rt_unpin
rt_unpin:
	ENTER_RT
	call rt_heap_acquire
	mov rax, [rip+rt_pin_count]
	lea rcx, [rip+rt_pins]
1:
	test rax, rax
	jz 2f
	dec rax
	cmp [rcx+8*rax], rdi
	jne 1b
	mov rdx, [rip+rt_pin_count]
	dec rdx
	mov r8, [rcx+8*rdx]
	mov [rcx+8*rax], r8
	mov [rip+rt_pin_count], rdx
2:
	mov qword ptr [rip+rt_heap_lock], 0
	ret

rt_heap_acquire:
	push rdi
1:
	mov eax, 1
	xchg [rip+rt_heap_lock], rax
	test rax, rax
	jz 3f
2:
	cmp dword ptr [rip+rt_stop], 0
	je 4f
	call rt_stop_here
4:
	pause
	cmp qword ptr [rip+rt_heap_lock], 0
	jne 2b
	jmp 1b
3:
	pop rdi
	ret

	.globl rt_panic_index
rt_panic_index:
	lea rdi, [rip+rt_msg_index]
	jmp rt_panic

rt_die:
	push rdi
	push rcx
	mov edi, 2
	call rt_write
	mov rdi, [rsp+8]
	mov rdx, [rdi]
	lea rsi, [rdi+8]
	mov edi, 2
	call rt_write
	mov edi, 2
	lea rsi, [rip+rt_nl]
	mov edx, 1
	call rt_write
	pop rdi
	jmp rt_exit

rt_panic:
	lea rsi, [rip+rt_panic_prefix]
	mov edx, 7
	mov ecx, 2
	jmp rt_die

rt_error:
	lea rsi, [rip+rt_error_prefix]
	mov edx, 7
	mov ecx, 1
	jmp rt_die

	.globl rt_panic_msg
rt_panic_msg:
	push rdi
	push rsi
	mov edi, 2
	lea rsi, [rip+rt_panic_prefix]
	mov edx, 7
	call rt_write
	pop rdx
	pop rsi
	mov edi, 2
	call rt_write
	mov edi, 2
	lea rsi, [rip+rt_nl]
	mov edx, 1
	call rt_write
	mov edi, 2
	jmp rt_exit

	.globl rt_panic_divzero
rt_panic_divzero:
	lea rdi, [rip+rt_msg_divzero]
	jmp rt_panic

rt_panic_oom:
	lea rdi, [rip+rt_msg_oom]
	jmp rt_panic

rt_gc_init:
	mov rsi, 17179869184
	call rt_mmap
	mov [rip+rt_heap_start], rax
	mov [rip+rt_heap_bump], rax
	mov rcx, 17179869184
	add rcx, rax
	mov [rip+rt_heap_end], rcx
	mov rsi, 268435456
	call rt_mmap
	mov [rip+rt_bitmap], rax
	mov rsi, 4194304
	call rt_mmap
	mov [rip+rt_summary1], rax
	mov rsi, 65536
	call rt_mmap
	mov [rip+rt_summary2], rax
	mov rsi, 8589934592
	call rt_mmap
	mov [rip+rt_mark_base], rax
	mov qword ptr [rip+rt_gc_threshold], 524288
	ret

rt_set_start:
	mov rax, rdi
	sub rax, [rip+rt_heap_start]
	shr rax, 3
	mov rcx, [rip+rt_bitmap]
	lock bts qword ptr [rcx], rax
	jmp rt_set_summaries

rt_set_summaries:
	shr rax, 6
	mov rcx, [rip+rt_summary1]
	bt qword ptr [rcx], rax
	jc 1f
	lock bts qword ptr [rcx], rax
1:
	shr rax, 6
	mov rcx, [rip+rt_summary2]
	bt qword ptr [rcx], rax
	jc 2f
	lock bts qword ptr [rcx], rax
2:
	ret

rt_clear_start:
	mov rax, rdi
	sub rax, [rip+rt_heap_start]
	shr rax, 3
	mov rcx, [rip+rt_bitmap]
	btr qword ptr [rcx], rax
	shr rax, 6
	cmp qword ptr [rcx+8*rax], 0
	jne 1f
	mov rcx, [rip+rt_summary1]
	btr qword ptr [rcx], rax
	shr rax, 6
	cmp qword ptr [rcx+8*rax], 0
	jne 1f
	mov rcx, [rip+rt_summary2]
	btr qword ptr [rcx], rax
1:
	ret

rt_free_chunk:
	mov rax, rsi
	shl rax, 3
	or rax, 2
	mov [rdi], rax
	lea rcx, [rip+rt_free_large]
	cmp rsi, 64
	jae 1f
	lea rcx, [rip+rt_free_small]
	lea rcx, [rcx+8*rsi]
1:
	mov rax, [rcx]
	mov [rdi+8], rax
	mov [rcx], rdi
	jmp rt_set_start

	.globl rt_argc
rt_argc:
	mov rax, [rip+rt_arg_count]
	ret

	.globl rt_argv
rt_argv:
	mov rax, [rip+rt_arg_values]
	mov rax, [rax+8*rdi]
	ret

	.globl rt_envv
rt_envv:
	mov rax, [rip+rt_env_values]
	test rax, rax
	jz 1f
	mov rax, [rax+8*rdi]
1:
	ret

	.globl rt_alloc
rt_alloc:
	ENTER_RT
	cmp rdi, 1
	adc rdi, 0
	cmp rdi, ALLOC_SMALL
	jbe rt_alloc_small
	push rsi
	call rt_heap_acquire
	pop rsi
	call rt_alloc_body
	mov qword ptr [rip+rt_heap_lock], 0
	ret

rt_alloc_small:
	lea rdx, [rdi+1]
1:
	TLS_GET r8, TLS_M
	mov rax, [r8+M_ALLOC_CUR]
	mov r9, [r8+M_ALLOC_END]
	test rax, rax
	jz 5f
	lea rcx, [rax+8*rdx]
	cmp rcx, r9
	ja 5f
	lea r10, [rcx+8]
	cmp r10, r9
	jne 2f
	mov rcx, r9
	inc rdx
2:
	mov [r8+M_ALLOC_CUR], rcx
	shl rdx, 3
	test rsi, rsi
	jnz 3f
	or rdx, 4
3:
	mov [rax], rdx
	mov r9, rax
	mov rcx, rax
	sub rcx, [rip+rt_heap_start]
	shr rcx, 3
	mov rdx, [rip+rt_bitmap]
	cmp rax, [r8+M_ALLOC_LO]
	jb 4f
	cmp rax, [r8+M_ALLOC_HI]
	jae 4f
	bts qword ptr [rdx], rcx
	jmp 6f
4:
	lock bts qword ptr [rdx], rcx
6:
	mov rax, rcx
	call rt_set_summaries
	lea rax, [r9+8]
	ret
5:
	push rdx
	push rsi
	mov rdi, rdx
	call rt_alloc_refill
	pop rsi
	pop rdx
	jmp 1b

rt_alloc_retire:
	test rdi, rdi
	jz 1f
	sub rsi, rdi
	shr rsi, 3
	cmp rsi, 2
	jb 1f
	jmp rt_free_chunk
1:
	ret

rt_alloc_refill:
	push rbx
	push r12
	push r13
	mov r12, rdi
	call rt_heap_acquire
	TLS_GET rbx, TLS_M
	mov rdi, [rbx+M_ALLOC_CUR]
	mov rsi, [rbx+M_ALLOC_END]
	mov qword ptr [rbx+M_ALLOC_CUR], 0
	mov qword ptr [rbx+M_ALLOC_END], 0
	call rt_alloc_retire
	mov rax, [rip+rt_alloc_since]
	add rax, ALLOC_BUFFER
	mov [rip+rt_alloc_since], rax
	cmp rax, [rip+rt_gc_threshold]
	jb 1f
	call rt_gc
1:
	lea rcx, [rip+rt_free_large]
2:
	mov rax, [rcx]
	test rax, rax
	jz 5f
	mov rdx, [rax]
	shr rdx, 3
	cmp rdx, r12
	jae 3f
	lea rcx, [rax+8]
	jmp 2b
3:
	mov r8, [rax+8]
	mov [rcx], r8
	mov r13, rdx
	cmp r13, ALLOC_BUFFER+2
	jb 8f
	push rax
	lea rdi, [rax+8*ALLOC_BUFFER]
	lea rsi, [r13-ALLOC_BUFFER]
	call rt_free_chunk
	pop rax
	mov r13, ALLOC_BUFFER
	jmp 8f
5:
	mov rcx, 63
6:
	cmp rcx, r12
	jb 7f
	lea rdx, [rip+rt_free_small]
	mov rax, [rdx+8*rcx]
	test rax, rax
	jnz 4f
	dec rcx
	jmp 6b
4:
	mov r8, [rax+8]
	mov [rdx+8*rcx], r8
	mov r13, rcx
	jmp 8f
7:
	mov r13, ALLOC_BUFFER
	mov rax, [rip+rt_heap_bump]
	lea rdx, [rax+8*ALLOC_BUFFER]
	cmp rdx, [rip+rt_heap_end]
	ja rt_panic_oom
	HEAP_COMMIT rdx
	mov [rip+rt_heap_bump], rdx
8:
	mov rdi, rax
	mov rcx, r13
	mov rdx, rax
	xor eax, eax
	rep stosq
	mov [rbx+M_ALLOC_CUR], rdx
	lea rax, [rdx+8*r13]
	mov [rbx+M_ALLOC_END], rax
	and rax, -512
	mov [rbx+M_ALLOC_HI], rax
	add rdx, 511
	and rdx, -512
	mov [rbx+M_ALLOC_LO], rdx
	mov qword ptr [rip+rt_heap_lock], 0
	pop r13
	pop r12
	pop rbx
	ret

rt_alloc_retire_all:
	push rbx
	push r12
	xor r12d, r12d
1:
	cmp r12, [rip+rt_nm]
	jae 2f
	lea rax, [rip+rt_allm]
	mov rbx, [rax+8*r12]
	inc r12
	mov rdi, [rbx+M_ALLOC_CUR]
	mov rsi, [rbx+M_ALLOC_END]
	mov qword ptr [rbx+M_ALLOC_CUR], 0
	mov qword ptr [rbx+M_ALLOC_END], 0
	call rt_alloc_retire
	jmp 1b
2:
	pop r12
	pop rbx
	ret

rt_alloc_body:
	push rsi
	cmp rdi, 1
	adc rdi, 0
	lea rsi, [rdi+1]
	mov rax, [rip+rt_alloc_since]
	add rax, rsi
	mov [rip+rt_alloc_since], rax
	cmp rax, [rip+rt_gc_threshold]
	jb 1f
	push rsi
	call rt_gc
	pop rsi
1:
	cmp rsi, 64
	jae 2f
	lea rcx, [rip+rt_free_small]
	mov rax, [rcx+8*rsi]
	test rax, rax
	jz 2f
	mov rdx, [rax+8]
	mov [rcx+8*rsi], rdx
	jmp 6f
2:
	lea rcx, [rip+rt_free_large]
3:
	mov rax, [rcx]
	test rax, rax
	jz 5f
	mov rdx, [rax]
	shr rdx, 3
	cmp rdx, rsi
	jae 4f
	lea rcx, [rax+8]
	jmp 3b
4:
	mov r8, [rax+8]
	mov [rcx], r8
	mov r9, rdx
	sub r9, rsi
	cmp r9, 2
	jb 7f
	push rax
	push rsi
	lea rdi, [rax+8*rsi]
	mov rsi, r9
	call rt_free_chunk
	pop rsi
	pop rax
	jmp 6f
7:
	mov rsi, rdx
	jmp 6f
5:
	mov rax, [rip+rt_heap_bump]
	lea rdx, [rax+8*rsi]
	cmp rdx, [rip+rt_heap_end]
	ja rt_panic_oom
	HEAP_COMMIT rdx
	mov [rip+rt_heap_bump], rdx
	push rax
	push rsi
	mov rdi, rax
	call rt_set_start
	pop rsi
	pop rax
6:
	mov rdx, rsi
	shl rdx, 3
	pop rcx
	test rcx, rcx
	jnz 8f
	or rdx, 4
8:
	mov [rax], rdx
	lea rdi, [rax+8]
	lea rcx, [rsi-1]
	mov rdx, rax
	xor eax, eax
	rep stosq
	lea rax, [rdx+8]
	ret

rt_find:
	mov r8, [rip+rt_heap_start]
	cmp rdi, r8
	jb 9f
	cmp rdi, [rip+rt_heap_bump]
	jae 9f
	mov rdx, rdi
	sub rdx, r8
	shr rdx, 3
	mov r9, [rip+rt_bitmap]
	mov rax, rdx
	shr rax, 6
	mov rax, [r9+8*rax]
	mov ecx, edx
	not ecx
	shl rax, cl
	test rax, rax
	jz 1f
	bsr rax, rax
	lea rax, [rax+rdx-63]
	jmp 8f
1:
	shr rdx, 6
	sub rdx, 1
	jb 9f
	mov r9, [rip+rt_summary1]
	mov rax, rdx
	shr rax, 6
	mov rax, [r9+8*rax]
	mov ecx, edx
	not ecx
	shl rax, cl
	test rax, rax
	jz 2f
	bsr rax, rax
	lea rax, [rax+rdx-63]
	jmp 7f
2:
	shr rdx, 6
	sub rdx, 1
	jb 9f
	mov r9, [rip+rt_summary2]
	mov rax, rdx
	shr rax, 6
	mov rax, [r9+8*rax]
	mov ecx, edx
	not ecx
	shl rax, cl
	test rax, rax
	jz 3f
	bsr rax, rax
	lea rax, [rax+rdx-63]
	jmp 6f
3:
	shr rdx, 6
4:
	sub rdx, 1
	jb 9f
	mov rax, [r9+8*rdx]
	test rax, rax
	jz 4b
	bsr rax, rax
	shl rdx, 6
	add rax, rdx
6:
	mov r9, [rip+rt_summary1]
	mov rdx, rax
	mov rax, [r9+8*rax]
	bsr rax, rax
	shl rdx, 6
	add rax, rdx
7:
	mov r9, [rip+rt_bitmap]
	mov rdx, rax
	mov rax, [r9+8*rax]
	bsr rax, rax
	shl rdx, 6
	add rax, rdx
8:
	lea rax, [r8+8*rax]
	ret
9:
	xor eax, eax
	ret

rt_mark:
	call rt_find
	test rax, rax
	jz 1f
	mov rdx, [rax]
	test edx, 3
	jnz 1f
	or rdx, 1
	mov [rax], rdx
	mov [r15], rax
	add r15, 8
1:
	ret

rt_gc:
	push rbx
	push r12
	push r13
	push r14
	push r15
	TLS_GET rax, TLS_G
	mov [rax+G_SCANLO], rsp
	call rt_world_stop
	call rt_alloc_retire_all
	mov r15, [rip+rt_mark_base]
	lea rdi, [rip+rt_pins]
	mov rsi, [rip+rt_pin_count]
	lea rsi, [rdi+8*rsi]
	call rt_scan_range
	xor r12d, r12d
1:
	cmp r12, [rip+rt_ngs]
	jae 6f
	mov rax, [rip+rt_allgs]
	mov rdi, [rax+8*r12]
	inc r12
	mov rax, [rdi+G_STATUS]
	cmp rax, G_RUNNABLE
	jb 1b
	cmp rax, G_WAITING
	ja 1b
	call rt_scan_g
	jmp 1b
6:
	cmp r15, [rip+rt_mark_base]
	jbe 8f
	sub r15, 8
	mov r12, [r15]
	test byte ptr [r12], 4
	jnz 6b
	mov r13, [r12]
	shr r13, 3
	mov r14d, 1
7:
	cmp r14, r13
	jae 6b
	mov rdi, [r12+8*r14]
	call rt_mark
	inc r14
	jmp 7b
8:
	call rt_sweep
	call rt_world_start
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

rt_scan_g:
	push rbx
	push r12
	push r13
	push r14
	push qword ptr [rdi+G_HI]
	mov rbx, rdi
	mov rdi, [rbx+G_CTX]
	call rt_mark
	cmp qword ptr [rbx+G_PC], 0
	jne 1f
	mov rdi, [rbx+G_SCANLO]
	mov rsi, [rbx+G_HI]
	call rt_scan_range
	jmp 8f
1:
	mov rdi, [rbx+G_SCANLO]
	mov rsi, [rbx+G_USERSP]
	call rt_scan_range
	mov r12, [rbx+G_PC]
	mov r13, [rbx+G_FP]
	mov r14, [rbx+G_USERSP]
2:
	test r13, r13
	jz 7f
	cmp r13, [rsp]
	jae 7f
	mov rdi, r12
	call rt_lookup_map
	test rax, rax
	jz 4f
	mov rbx, rax
	mov r12, [rbx]
	cmp r12, -1
	je 6f
3:
	test r12, r12
	jz 5f
	add rbx, 8
	mov rax, [rbx]
	mov rdi, [r13+rax]
	call rt_mark
	dec r12
	jmp 3b
4:
	mov rdi, r14
	mov rsi, r13
	call rt_scan_range
5:
	lea r14, [r13+16]
	mov r12, [r13+8]
	mov r13, [r13]
	jmp 2b
6:
	mov rdi, [r13-8]
	mov rsi, [r13-16]
	call rt_scan_range
	lea rdi, [r13+16]
	mov rsi, [r13-24]
	call rt_scan_range
	mov r14, [r13-24]
	mov r12, [r13+8]
	mov r13, [r13]
	jmp 2b
7:
	mov rdi, r14
	mov rsi, [rsp]
	call rt_scan_range
8:
	add rsp, 8
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

rt_world_stop:
	push rbx
	push r12
	mov dword ptr [rip+rt_stop], 1
	TLS_GET r12, TLS_M
	xor ebx, ebx
1:
	cmp rbx, [rip+rt_nm]
	jae 2f
	lea rax, [rip+rt_allm]
	mov rdi, [rax+8*rbx]
	mov rax, [rdi+M_FLAG]
	mov byte ptr [rax], 1
	inc rbx
	jmp 1b
2:
	xor ebx, ebx
3:
	cmp rbx, [rip+rt_nm]
	jae 6f
	lea rax, [rip+rt_allm]
	mov rdi, [rax+8*rbx]
	inc rbx
	cmp rdi, r12
	je 3b
4:
	mov rax, [rdi+M_STATE]
	cmp rax, RUNNING
	jne 5f
	pause
	jmp 4b
5:
	test eax, HELD
	jnz 3b
	mov rcx, rax
	or rcx, HELD
	lock cmpxchg [rdi+M_STATE], rcx
	jne 4b
	jmp 3b
6:
	pop r12
	pop rbx
	ret

rt_world_start:
	push rbx
	TLS_GET r8, TLS_M
	xor ebx, ebx
1:
	cmp rbx, [rip+rt_nm]
	jae 2f
	lea rax, [rip+rt_allm]
	mov rdi, [rax+8*rbx]
	inc rbx
	cmp rdi, r8
	je 1b
	lock and qword ptr [rdi+M_STATE], -9
	jmp 1b
2:
	mov dword ptr [rip+rt_stop], 0
	lea rdi, [rip+rt_stop]
	mov esi, 0x7fffffff
	call rt_futex_wake
	TLS_CLEAR_FLAG
	pop rbx
	ret

rt_scan_range:

	push rbx
	push r12
	mov rbx, rdi
	mov r12, rsi
1:
	cmp rbx, r12
	jae 2f
	mov rdi, [rbx]
	call rt_mark
	add rbx, 8
	jmp 1b
2:
	pop r12
	pop rbx
	ret

rt_lookup_map:
	lea r8, [rip+rt_maps]
	mov rcx, [r8]
	add r8, 8
	xor eax, eax
1:
	cmp rax, rcx
	jae 3f
	lea rdx, [rax+rcx]
	shr rdx, 1
	mov r9, rdx
	shl r9, 4
	cmp rdi, [r8+r9]
	je 2f
	jb 4f
	lea rax, [rdx+1]
	jmp 1b
4:
	mov rcx, rdx
	jmp 1b
2:
	mov rax, [r8+r9+8]
	ret
3:
	xor eax, eax
	ret

rt_sweep:
	lea rdi, [rip+rt_free_small]
	mov ecx, 64
	xor eax, eax
	rep stosq
	mov qword ptr [rip+rt_free_large], 0
	mov r12, [rip+rt_heap_start]
	xor r13d, r13d
	xor r14d, r14d
	xor ebx, ebx
1:
	cmp r12, [rip+rt_heap_bump]
	jae 5f
	mov rax, [r12]
	mov rcx, rax
	shr rcx, 3
	test eax, 1
	jz 2f
	and rax, -2
	mov [r12], rax
	add rbx, rcx
	test r13, r13
	jz 4f
	push rcx
	mov rdi, r13
	mov rsi, r14
	call rt_free_chunk
	pop rcx
	xor r13d, r13d
	jmp 4f
2:
	test r13, r13
	jnz 3f
	mov r13, r12
	mov r14, rcx
	jmp 4f
3:
	add r14, rcx
	push rcx
	mov rdi, r12
	call rt_clear_start
	pop rcx
4:
	lea r12, [r12+8*rcx]
	jmp 1b
5:
	test r13, r13
	jz 6f
	mov [rip+rt_heap_bump], r13
	mov rdi, r13
	call rt_clear_start
6:
	mov rax, rbx
	mov rcx, 524288
	cmp rax, rcx
	cmovb rax, rcx
	mov [rip+rt_gc_threshold], rax
	mov qword ptr [rip+rt_alloc_since], 0
	ret

	.section .rodata
	.p2align 3
rt_exp_max:
	.double 709.782712893384
rt_exp_min:
	.double -745.1332191019411
rt_exp_log2e:
	.double 1.4426950408889634
rt_exp_ln2hi:
	.double 6.93147180369123816490e-01
rt_exp_ln2lo:
	.double 1.90821492927058770002e-10
rt_exp_coef:
	.double 1.6059043836821613e-10
	.double 2.08767569878681e-09
	.double 2.505210838544172e-08
	.double 2.755731922398589e-07
	.double 2.7557319223985893e-06
	.double 2.48015873015873e-05
	.double 0.0001984126984126984
	.double 0.001388888888888889
	.double 0.008333333333333333
	.double 0.041666666666666664
	.double 0.16666666666666666
	.double 0.5
	.double 1.0
	.double 1.0

	.text
	.globl rt_exp
rt_exp:
	movq xmm0, rdi
	ucomisd xmm0, xmm0
	jp 3f
	ucomisd xmm0, [rip+rt_exp_max]
	ja 4f
	movsd xmm1, [rip+rt_exp_min]
	ucomisd xmm1, xmm0
	ja 5f
	movsd xmm1, xmm0
	mulsd xmm1, [rip+rt_exp_log2e]
	cvtsd2si rax, xmm1
	cvtsi2sd xmm2, rax
	movsd xmm3, xmm2
	mulsd xmm3, [rip+rt_exp_ln2hi]
	subsd xmm0, xmm3
	mulsd xmm2, [rip+rt_exp_ln2lo]
	subsd xmm0, xmm2
	lea rcx, [rip+rt_exp_coef]
	movsd xmm1, [rcx]
	mov edx, 1
1:
	mulsd xmm1, xmm0
	addsd xmm1, [rcx+8*rdx]
	inc edx
	cmp edx, 14
	jb 1b
	mov rdx, rax
	sar rdx, 1
	sub rax, rdx
	add rdx, 1023
	shl rdx, 52
	movq xmm2, rdx
	mulsd xmm1, xmm2
	add rax, 1023
	shl rax, 52
	movq xmm2, rax
	mulsd xmm1, xmm2
	movq rax, xmm1
	ret
3:
	mov rax, rdi
	ret
4:
	mov rax, 0x7ff0000000000000
	ret
5:
	xor eax, eax
	ret

	.data
	.globl rt_avx2
rt_avx2:
	.byte 0

	.text
	.globl rt_cpu_init
rt_cpu_init:
	push rbx
	push r12
	mov r12, rdi
	call rt_vdso_init
	mov eax, 1
	xor ecx, ecx
	cpuid
	mov r8d, ecx
	and r8d, 0x18000000
	cmp r8d, 0x18000000
	jne 3f
	xor ecx, ecx
	xgetbv
	and eax, 6
	cmp eax, 6
	jne 3f
	mov eax, 7
	xor ecx, ecx
	cpuid
	test ebx, 32
	jz 3f
	mov byte ptr [rip+rt_avx2], 1
3:
	test r12, r12
	jz 9f
4:
	mov rsi, [r12]
	test rsi, rsi
	jz 9f
	lea rdi, [rip+rt_simd_env]
	mov ecx, 14
	repe cmpsb
	jne 5f
	mov byte ptr [rip+rt_avx2], 0
	jmp 9f
5:
	add r12, 8
	jmp 4b
9:
	pop r12
	pop rbx
	ret
