	.section .rodata
rt_env_gc:
	.asciz "LOW_GC"
rt_env_gctrace:
	.asciz "LOW_GCTRACE"
rt_trace_head:
	.asciz "gc "
rt_trace_labels:
	.asciz " pause_us="
	.asciz " mark_us="
	.asciz " sweep_us="
	.asciz " live_kb="
	.asciz " heap_kb="
	.asciz " slow_allocs="
	.asciz " refills="

	.bss
	.p2align 3
rt_gc_mode:
	.zero 8
rt_gc_trace:
	.zero 8
rt_gc_count:
	.zero 8
rt_gc_ticks:
	.zero 8*5
rt_trace_values:
	.zero 8*7
rt_stat_slow:
	.zero 8
rt_stat_refills:
	.zero 8

	.text
rt_gc_env:
	lea rdi, [rip+rt_env_gctrace]
	call rt_env_byte
	test eax, eax
	jz 2f
	cmp eax, 48
	je 2f
	mov byte ptr [rip+rt_gc_trace], 1
2:
	lea rdi, [rip+rt_env_gc]
	call rt_env_byte
	xor ecx, ecx
	cmp eax, 111
	mov edx, GC_OFF
	cmove ecx, edx
	cmp eax, 115
	mov edx, GC_STRESS
	cmove ecx, edx
	mov byte ptr [rip+rt_gc_mode], cl
	ret

rt_gc_set_threshold:
	mov rcx, 524288
	cmp rax, rcx
	cmovb rax, rcx
	movzx ecx, byte ptr [rip+rt_gc_mode]
	cmp ecx, GC_OFF
	jne 1f
	mov rax, -1
	jmp 2f
1:
	cmp ecx, GC_STRESS
	jne 2f
	mov eax, STRESS_WORDS
2:
	mov [rip+rt_gc_threshold], rax
	ret

rt_gc_stamp:
	cmp byte ptr [rip+rt_gc_trace], 0
	jne 1f
	ret
1:
	push rax
	push rcx
	push rdx
	push rsi
	push rdi
	push r8
	push r9
	push r10
	push r11
	mov edi, CLOCK_MONOTONIC
	call rt_clock_nanos
	mov rdi, [rsp+32]
	lea rcx, [rip+rt_gc_ticks]
	mov [rcx+8*rdi], rax
	pop r11
	pop r10
	pop r9
	pop r8
	pop rdi
	pop rsi
	pop rdx
	pop rcx
	pop rax
	ret

rt_gc_stats_collect:
	xor eax, eax
	xor ecx, ecx
	xor edx, edx
1:
	cmp rcx, [rip+rt_nm]
	jae 2f
	lea rsi, [rip+rt_allm]
	mov rsi, [rsi+8*rcx]
	inc rcx
	add rax, [rsi+M_SLOWS]
	add rdx, [rsi+M_REFILLS]
	mov qword ptr [rsi+M_SLOWS], 0
	mov qword ptr [rsi+M_REFILLS], 0
	jmp 1b
2:
	add rax, [rip+rt_stat_slow]
	add rdx, [rip+rt_stat_refills]
	mov qword ptr [rip+rt_stat_slow], 0
	mov qword ptr [rip+rt_stat_refills], 0
	lea rcx, [rip+rt_trace_values]
	mov [rcx+40], rax
	mov [rcx+48], rdx
	ret

rt_gc_trace_print:
	push rbx
	push r12
	sub rsp, 256
	lea rcx, [rip+rt_gc_ticks]
	lea rdx, [rip+rt_trace_values]
	mov rax, [rcx+8*TICK_RESUMED]
	sub rax, [rcx+8*TICK_ENTER]
	mov [rdx], rax
	mov rax, [rcx+8*TICK_MARKED]
	sub rax, [rcx+8*TICK_STOPPED]
	mov [rdx+8], rax
	mov rax, [rcx+8*TICK_SWEPT]
	sub rax, [rcx+8*TICK_MARKED]
	mov [rdx+16], rax
	mov ecx, 1000
	xor r8d, r8d
1:
	mov rax, [rdx+8*r8]
	push rdx
	xor edx, edx
	div rcx
	pop rdx
	mov [rdx+8*r8], rax
	inc r8d
	cmp r8d, 3
	jb 1b
	mov rax, [rip+rt_live_words]
	shl rax, 3
	shr rax, 10
	mov [rdx+24], rax
	mov rax, [rip+rt_heap_bump]
	sub rax, [rip+rt_heap_start]
	shr rax, 10
	mov [rdx+32], rax
	mov rdi, rsp
	lea rsi, [rip+rt_trace_head]
	call rt_append_str
	mov rsi, [rip+rt_gc_count]
	call rt_append_u64
	lea rbx, [rip+rt_trace_labels]
	lea r12, [rip+rt_trace_values]
	xor ecx, ecx
2:
	push rcx
	mov rsi, rbx
	call rt_append_str
	mov rbx, rsi
	mov rsi, [r12]
	add r12, 8
	call rt_append_u64
	pop rcx
	inc ecx
	cmp ecx, 7
	jb 2b
	mov byte ptr [rdi], 10
	inc rdi
	mov rdx, rdi
	mov rsi, rsp
	sub rdx, rsi
	mov edi, 2
	call rt_write
	add rsp, 256
	pop r12
	pop rbx
	ret

rt_append_str:
1:
	mov al, [rsi]
	inc rsi
	test al, al
	jz 2f
	mov [rdi], al
	inc rdi
	jmp 1b
2:
	ret

rt_append_u64:
	sub rsp, 24
	lea r8, [rsp+24]
	mov rax, rsi
	mov ecx, 10
1:
	xor edx, edx
	div rcx
	add dl, 48
	dec r8
	mov [r8], dl
	test rax, rax
	jnz 1b
2:
	lea rax, [rsp+24]
	cmp r8, rax
	jae 3f
	mov dl, [r8]
	mov [rdi], dl
	inc rdi
	inc r8
	jmp 2b
3:
	add rsp, 24
	ret
