
	.bss
	.p2align 3
	.globl rt_patch_count
rt_patch_count:
	.zero 8
	.globl rt_patch_list
rt_patch_list:
	.zero PATCH_BYTES

	.section .rodata
	MSG_BEGIN rt_msg_hot
	.ascii "cannot reserve hot reload space"
	MSG_END

	.text
rt_hot_init:
	mov edi, PATCH_SIGNAL
	lea rsi, [rip+rt_hot_handler]
	mov edx, 0x14000000
	call rt_sigaction
	mov rdi, HOT_BASE
	mov rsi, HOT_SIZE
	mov edx, 5
	mov r10d, 0x100022
	mov r8, -1
	xor r9d, r9d
	mov eax, 9
	syscall
	cmp rax, rdi
	jne 1f
	ret
1:
	lea rdi, [rip+rt_msg_hot]
	jmp rt_panic

rt_hot_handler:
	mov rcx, [rip+rt_patch_count]
	lea rsi, [rip+rt_patch_list]
	lea rdi, [rip+fn_table]
1:
	test rcx, rcx
	jz 2f
	mov rax, [rsi]
	mov rdx, [rsi+8]
	mov [rdi+8*rax], rdx
	add rsi, PATCH_ENTRY
	dec rcx
	jmp 1b
2:
	mov qword ptr [rip+rt_patch_count], 0
	ret
