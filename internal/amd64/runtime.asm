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
rt_msg_divzero:
	.quad 22
	.ascii "integer divide by zero"

	.text
	.globl _start
_start:
	call fn_main
	xor edi, edi
	mov eax, 60
	syscall

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

rt_print_str:
	mov rdx, [rdi]
	lea rsi, [rdi+8]
	mov edi, 1
	call rt_write
	mov edi, 1
	lea rsi, [rip+rt_nl]
	mov edx, 1
	jmp rt_write

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

rt_panic:
	push rdi
	mov edi, 2
	lea rsi, [rip+rt_panic_prefix]
	mov edx, 7
	call rt_write
	mov rdi, [rsp]
	mov rdx, [rdi]
	lea rsi, [rdi+8]
	mov edi, 2
	call rt_write
	mov edi, 2
	lea rsi, [rip+rt_nl]
	mov edx, 1
	call rt_write
	mov edi, 2
	mov eax, 60
	syscall

rt_panic_divzero:
	lea rdi, [rip+rt_msg_divzero]
	jmp rt_panic
