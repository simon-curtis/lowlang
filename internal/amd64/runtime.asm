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
rt_msg_index:
	.quad 18
	.ascii "index out of range"
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
rt_user_pc:
	.zero 8
rt_user_sp:
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
	mov rdx, rsi
	mov rsi, rdi
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
	xor eax, eax
	cmp rsi, rcx
	jne 1f
	mov rsi, rdx
	test rcx, rcx
	repe cmpsb
	sete al
1:
	ret

	.globl rt_grow
rt_grow:
	mov rax, [rsp]
	mov [rip+rt_user_pc], rax
	lea rax, [rsp+8]
	mov [rip+rt_user_sp], rax
	push rdi
	push rsi
	mov rsi, rcx
	lea rdi, [rdx+7]
	shr rdi, 3
	call rt_alloc_body
	pop rcx
	pop rsi
	mov rdi, rax
	rep movsb
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

	.globl rt_alloc
rt_alloc:
	mov rax, [rsp]
	mov [rip+rt_user_pc], rax
	lea rax, [rsp+8]
	mov [rip+rt_user_sp], rax
rt_alloc_body:
	push rsi
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
	mov rdi, rsp
	mov rsi, [rip+rt_user_sp]
	call rt_scan_range
	mov r12, [rip+rt_user_pc]
	mov r13, rbp
	mov r14, [rip+rt_user_sp]
1:
	test r13, r13
	jz 3f
	cmp r13, [rip+rt_stack_top]
	jae 3f
	mov rdi, r12
	call rt_lookup_map
	test rax, rax
	jz 2f
	mov rbx, rax
	mov r12, [rbx]
5:
	test r12, r12
	jz 4f
	add rbx, 8
	mov rax, [rbx]
	mov rdi, [r13+rax]
	call rt_mark
	dec r12
	jmp 5b
2:
	mov rdi, r14
	mov rsi, r13
	call rt_scan_range
4:
	lea r14, [r13+16]
	mov r12, [r13+8]
	mov r13, [r13]
	jmp 1b
3:
	mov rdi, r14
	mov rsi, [rip+rt_stack_top]
	call rt_scan_range
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
	pop r15
	pop r14
	pop r13
	pop r12
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
