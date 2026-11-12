	.macro TLS_GET reg, slot
	mov \reg, qword ptr fs:[\slot]
	.endm

	.macro TLS_SET slot, reg
	mov qword ptr fs:[\slot], \reg
	.endm

	.macro TLS_CLEAR_FLAG
	mov byte ptr fs:[TLS_FLAG], 0
	.endm

	.macro TLS_POLL
	cmp byte ptr fs:[TLS_FLAG], 0
	.endm

	.macro TEB_ENTER_OS g, m
	.endm

	.macro TEB_ENTER_G g, m
	.endm

	.macro TEB_ENTER_CURRENT_G m
	.endm

	.macro HEAP_COMMIT reg
	.endm

	.macro PROGRAM_ENTRY entry
.if RT_DYNAMIC
	.globl \entry
\entry:
	xor ebp, ebp
	mov r9, rdx
	pop rsi
	mov rdx, rsp
	and rsp, -16
	push rax
	push rsp
	xor r8d, r8d
	xor ecx, ecx
	lea rdi, [rip+main]
	call qword ptr [rip+__libc_start_main@GOTPCREL]
	hlt
	.globl main
main:
	push rbp
	xor ebp, ebp
	mov [rip+rt_arg_count], rdi
	mov [rip+rt_arg_values], rsi
	mov [rip+rt_env_values], rdx
	mov rdi, rdx
.else
	.globl \entry
\entry:
	mov rax, [rsp]
	mov [rip+rt_arg_count], rax
	lea rcx, [rsp+8]
	mov [rip+rt_arg_values], rcx
	lea rdi, [rsp+8*rax+16]
	mov [rip+rt_env_values], rdi
.endif
	PROGRAM_BOOT
	.endm

	.macro PROGRAM_EXIT
.if RT_DYNAMIC
	call exit
.else
	mov eax, 231
	syscall
.endif
	.endm
