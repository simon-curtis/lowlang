	.intel_syntax noprefix

	.section .rodata
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
rt_msg_oom:
	.quad 13
	.ascii "out of memory"

	.bss
	.p2align 3
rt_stack_top:
	.zero 8
rt_heap_start:
	.zero 8
rt_heap_end:
	.zero 8
rt_heap_bump:
	.zero 8
rt_bitmap:
	.zero 8
rt_mark_base:
	.zero 8
rt_alloc_since:
	.zero 8
rt_gc_threshold:
	.zero 8
rt_free_large:
	.zero 8
rt_free_small:
	.zero 512

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
	mov rdx, [rdi]
	lea rsi, [rdi+8]
	mov edi, 1
	call rt_write
	mov edi, 1
	lea rsi, [rip+rt_nl]
	mov edx, 1
	jmp rt_write

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
	mov rcx, [rdi]
	xor eax, eax
	cmp rcx, [rsi]
	jne 1f
	add rdi, 8
	add rsi, 8
	test rcx, rcx
	repe cmpsb
	sete al
1:
	ret

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
	mov eax, 60
	syscall

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

	.globl rt_panic_divzero
rt_panic_divzero:
	lea rdi, [rip+rt_msg_divzero]
	jmp rt_panic

rt_panic_oom:
	lea rdi, [rip+rt_msg_oom]
	jmp rt_panic

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
	bts qword ptr [rcx], rax
	ret

rt_clear_start:
	mov rax, rdi
	sub rax, [rip+rt_heap_start]
	shr rax, 3
	mov rcx, [rip+rt_bitmap]
	btr qword ptr [rcx], rax
	ret

rt_free_chunk:
	mov rax, rsi
	shl rax, 2
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

	.globl rt_alloc
rt_alloc:
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
	shr rdx, 2
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
	mov [rip+rt_heap_bump], rdx
	push rax
	push rsi
	mov rdi, rax
	call rt_set_start
	pop rsi
	pop rax
6:
	mov rdx, rsi
	shl rdx, 2
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
	jb 3f
	cmp rdi, [rip+rt_heap_bump]
	jae 3f
	mov rdx, rdi
	sub rdx, r8
	shr rdx, 3
	mov r9, [rip+rt_bitmap]
	mov rcx, rdx
	and ecx, 63
	shr rdx, 6
	mov rax, [r9+8*rdx]
	xor ecx, 63
	shl rax, cl
	test rax, rax
	jz 1f
	bsr rax, rax
	sub rax, rcx
	jmp 2f
1:
	test rdx, rdx
	jz 3f
	dec rdx
	mov rax, [r9+8*rdx]
	test rax, rax
	jz 1b
	bsr rax, rax
2:
	shl rdx, 6
	add rax, rdx
	lea rax, [r8+8*rax]
	ret
3:
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
	mov r15, [rip+rt_mark_base]
	mov r12, rsp
	mov r13, [rip+rt_stack_top]
1:
	cmp r12, r13
	jae 2f
	mov rdi, [r12]
	call rt_mark
	add r12, 8
	jmp 1b
2:
	cmp r15, [rip+rt_mark_base]
	jbe 4f
	sub r15, 8
	mov r12, [r15]
	mov r13, [r12]
	shr r13, 2
	mov r14d, 1
3:
	cmp r14, r13
	jae 2b
	mov rdi, [r12+8*r14]
	call rt_mark
	inc r14
	jmp 3b
4:
	call rt_sweep
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
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
	shr rcx, 2
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
