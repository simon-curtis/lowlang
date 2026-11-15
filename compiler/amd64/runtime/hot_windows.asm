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
	push rbp
	mov rbp, rsp
	and rsp, -16
	sub rsp, 32
	mov rcx, HOT_BASE
	mov edx, HOT_SIZE
	mov r8d, 0x3000
	mov r9d, 0x20
	WIN VirtualAlloc
	mov rsp, rbp
	pop rbp
	mov rcx, HOT_BASE
	cmp rax, rcx
	jne 1f
	ret
1:
	lea rdi, [rip+rt_msg_hot]
	jmp rt_panic

	.globl rt_hot_handler
rt_hot_handler:
	mov r8, [rip+rt_patch_count]
	lea r9, [rip+rt_patch_list]
	lea r10, [rip+fn_table]
1:
	test r8, r8
	jz 2f
	mov rax, [r9]
	mov rdx, [r9+8]
	mov [r10+8*rax], rdx
	add r9, PATCH_ENTRY
	dec r8
	jmp 1b
2:
	mov qword ptr [rip+rt_patch_count], 0
	xor eax, eax
	ret
