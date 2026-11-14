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
rt_region_start:
	.zero 8
rt_region_hits:
	.zero 8
rt_region_bump:
	.zero 8
rt_region_list:
	.zero 8
rt_region_count:
	.zero 8
rt_region_links:
	.zero 8
rt_rq:
	.zero 8
rt_rq_count:
	.zero 8
rt_rp:
	.zero 8
rt_rp_count:
	.zero 8*21

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

rt_region_init:
	mov rdi, [rip+rt_region_start]
	mov rsi, 68719476736
	call rt_region_decommit
	mov rsi, 8388608
	call rt_mmap_stack
	mov [rip+rt_region_list], rax
	mov rsi, 8388608
	call rt_mmap_stack
	mov [rip+rt_rq], rax
	mov rsi, 16777216
	call rt_mmap_stack
	mov [rip+rt_rp], rax
	mov rsi, 131072
	call rt_mmap_stack
	mov [rip+rt_region_hits], rax
	mov rsi, 16*REGION_UNITS
	call rt_mmap_stack
	mov [rip+rt_region_links], rax
	ret

rt_region_hits_clear:
	mov rcx, [rip+rt_region_bump]
	test rcx, rcx
	jz 1f
	add rcx, 63
	shr rcx, 6
	mov rdi, [rip+rt_region_hits]
	xor eax, eax
	rep stosq
1:
	ret

rt_region_pool_base:
	mov ecx, 21
	sub ecx, esi
	mov edx, 1
	shl rdx, cl
	mov rax, 2097152
	sub rax, rdx
	mov rdx, [rip+rt_rp]
	lea rdx, [rdx+8*rax]
	ret

rt_region_pool_push:
	call rt_region_pool_base
	lea rcx, [rip+rt_rp_count]
	mov rax, [rcx+8*rsi]
	mov [rdx+8*rax], rdi
	inc rax
	mov [rcx+8*rsi], rax
	ret

rt_region_pool_pop:
	lea rcx, [rip+rt_rp_count]
	cmp qword ptr [rcx+8*rsi], 0
	je 1f
	call rt_region_pool_base
	lea rcx, [rip+rt_rp_count]
	mov rax, [rcx+8*rsi]
	dec rax
	mov [rcx+8*rsi], rax
	mov rax, [rdx+8*rax]
	ret
1:
	mov rax, -1
	ret

rt_region_reclaim:
	push rbx
	push r12
	push r13
	push r14
	xor ebx, ebx
	xor r12d, r12d
	mov r13, [rip+rt_rq]
	mov r14, [rip+rt_region_hits]
1:
	cmp rbx, [rip+rt_rq_count]
	jae 5f
	mov rax, [r13+8*rbx]
	inc rbx
	mov edi, eax
	mov rsi, rax
	shr rsi, 32
	mov ecx, esi
	mov r8d, 1
	shl r8, cl
	lea r9, [rdi+r8]
	mov r10, rdi
2:
	cmp r10, r9
	jae 4f
	bt qword ptr [r14], r10
	jc 3f
	inc r10
	jmp 2b
3:
	mov [r13+8*r12], rax
	inc r12
	jmp 1b
4:
	call rt_region_pool_push
	jmp 1b
5:
	mov [rip+rt_rq_count], r12
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

rt_scan_regions:
	push rbx
	push r12
	push r13
	xor ebx, ebx
1:
	cmp rbx, [rip+rt_region_count]
	jae 5f
	mov rax, [rip+rt_region_list]
	mov r12, [rax+8*rbx]
	inc rbx
	mov r13, [r12+R_CHUNKS]
2:
	test r13, r13
	jz 1b
	test byte ptr [r13+CH_KIND], 1
	jz 4f
	lea rdi, [r13+CH_SIZE]
	mov rsi, [r13+CH_USED]
	cmp r13, [r12+R_SCAN_CHUNK]
	jne 3f
	mov rsi, [r12+R_SCAN_BUMP]
3:
	call rt_scan_range
4:
	mov r13, [r13+CH_NEXT]
	jmp 2b
5:
	pop r13
	pop r12
	pop rbx
	ret

rt_region_fill:
	push rax
1:
	cmp r9, r10
	jae 2f
	mov rdi, r9
	xor esi, esi
	call rt_region_pool_push
	inc r9
	jmp 1b
2:
	pop rax
	ret

rt_region_keep_head:
	TLS_GET rcx, TLS_M
	cmp edi, REGION_FIRST
	jne 1f
	lea rcx, [rcx+M_KEEP_FIRST]
	mov edx, REGION_KEEP_FIRST
	ret
1:
	cmp edi, REGION_NEXT
	jne 2f
	lea rcx, [rcx+M_KEEP_NEXT]
	mov edx, REGION_KEEP_NEXT
	ret
2:
	xor ecx, ecx
	ret

rt_region_link:
	mov rax, rdi
	sub rax, [rip+rt_region_start]
	shr rax, REGION_SHIFT
	shl rax, 4
	add rax, [rip+rt_region_links]
	ret

rt_region_unkeep:
	call rt_region_keep_head
	test rcx, rcx
	jz 1f
	mov rax, [rcx]
	test rax, rax
	jz 2f
	push rax
	mov rdi, rax
	call rt_region_link
	mov rdx, [rax]
	mov [rcx], rdx
	pop rax
	ret
1:
	xor eax, eax
2:
	ret

rt_region_chunk:
	push rbx
	push r12
	push r13
	mov ebx, edi
	mov r12d, esi
	call rt_region_unkeep
	test rax, rax
	jnz 4f
	call rt_heap_acquire
	xor r13d, r13d
1:
	mov esi, ebx
	call rt_region_pool_pop
	cmp rax, -1
	jne 3f
	mov r9, [rip+rt_region_bump]
	mov ecx, ebx
	mov edx, 1
	shl rdx, cl
	lea rax, [r9+rdx-1]
	mov r8, rdx
	neg r8
	and rax, r8
	add rdx, rax
	cmp rdx, REGION_UNITS
	ja 2f
	mov [rip+rt_region_bump], rdx
	mov r10, rax
	call rt_region_fill
	jmp 3f
2:
	test r13d, r13d
	jnz 9f
	inc r13d
	call rt_gc
	jmp 1b
9:
	lea rdi, [rip+rt_msg_region_full]
	jmp rt_panic
3:
	mov qword ptr [rip+rt_heap_lock], 0
	shl rax, REGION_SHIFT
	add rax, [rip+rt_region_start]
4:
	mov rdi, rax
	mov ecx, ebx
	mov esi, 65536
	shl rsi, cl
	push rdi
	call rt_region_commit
	pop rax
	mov byte ptr [rax+CH_CLASS], bl
	mov byte ptr [rax+CH_KIND], r12b
	pop r13
	pop r12
	pop rbx
	ret

	.macro REGION_REFILL chunk, bump, limit, kind
	mov rax, [rbx+\chunk]
	test rax, rax
	jz 1f
	mov rcx, [rbx+\bump]
	mov [rax+CH_USED], rcx
	mov edi, REGION_NEXT
	jmp 2f
1:
	mov edi, REGION_FIRST
2:
	mov esi, \kind
	call rt_region_chunk
	mov rcx, [rbx+R_CHUNKS]
	mov [rax+CH_NEXT], rcx
	mov [rbx+R_CHUNKS], rax
	mov [rbx+\chunk], rax
	lea rcx, [rax+CH_SIZE]
	mov [rbx+\bump], rcx
	movzx ecx, byte ptr [rax+CH_CLASS]
	mov edx, 65536
	shl rdx, cl
	add rdx, rax
	mov [rbx+\limit], rdx
	.endm

rt_region_alloc:
	mov rcx, [rax+G_REGION]
	lea rdx, [rdi+1]
	shl rdx, 3
rt_region_take:
	test rsi, rsi
	jz 2f
	mov rax, [rcx+R_SCAN_BUMP]
	lea r8, [rax+rdx]
	cmp r8, [rcx+R_SCAN_END]
	ja 3f
	mov [rcx+R_SCAN_BUMP], r8
	mov [rax], rdx
	add rax, 8
	ret
2:
	mov rax, [rcx+R_NOSCAN_BUMP]
	lea r8, [rax+rdx]
	cmp r8, [rcx+R_NOSCAN_END]
	ja 3f
	mov [rcx+R_NOSCAN_BUMP], r8
	or rdx, 4
	mov [rax], rdx
	add rax, 8
	ret
3:
	push rbx
	push r12
	push r13
	mov rbx, rcx
	mov r12, rdx
	mov r13, rsi
	cmp r12, REGION_LARGE
	ja 5f
	test r13, r13
	jz 4f
	REGION_REFILL R_SCAN_CHUNK, R_SCAN_BUMP, R_SCAN_END, 1
	jmp 8f
4:
	REGION_REFILL R_NOSCAN_CHUNK, R_NOSCAN_BUMP, R_NOSCAN_END, 0
8:
	mov rcx, rbx
	mov rdx, r12
	mov rsi, r13
	pop r13
	pop r12
	pop rbx
	jmp rt_region_take
5:
	xor edi, edi
	lea rax, [r12+CH_SIZE+65535]
	shr rax, REGION_SHIFT
	dec rax
	jz 6f
	bsr rdi, rax
	inc edi
6:
	xor esi, esi
	test r13, r13
	jz 7f
	mov esi, 1
7:
	call rt_region_chunk
	mov rcx, [rbx+R_CHUNKS]
	mov [rax+CH_NEXT], rcx
	mov [rbx+R_CHUNKS], rax
	lea rdx, [rax+CH_SIZE]
	lea rcx, [rdx+r12]
	mov [rax+CH_USED], rcx
	mov rcx, r12
	test r13, r13
	jnz 9f
	or rcx, 4
9:
	mov [rdx], rcx
	lea rax, [rdx+8]
	pop r13
	pop r12
	pop rbx
	ret

	.globl rt_region_enter
rt_region_enter:
	ENTER_RT
	push rbx
	mov edi, REGION_FIRST
	xor esi, esi
	call rt_region_chunk
	lea rbx, [rax+CH_SIZE]
	mov [rbx+R_CHUNKS], rax
	mov [rbx+R_NOSCAN_CHUNK], rax
	lea rcx, [rbx+R_SIZE]
	mov [rbx+R_NOSCAN_BUMP], rcx
	add rax, 65536
	mov [rbx+R_NOSCAN_END], rax
	call rt_heap_acquire
	mov rcx, [rip+rt_region_count]
	mov rdx, [rip+rt_region_list]
	mov [rdx+8*rcx], rbx
	mov [rbx+R_INDEX], rcx
	inc rcx
	mov [rip+rt_region_count], rcx
	mov qword ptr [rip+rt_heap_lock], 0
	TLS_GET rax, TLS_G
	mov rcx, [rax+G_REGION]
	mov [rbx+R_PARENT], rcx
	mov [rax+G_REGION], rbx
	call rt_alloc_hold
	mov rax, rbx
	pop rbx
	ret

	.globl rt_region_leave
rt_region_leave:
	TLS_GET rax, TLS_G
	mov rcx, [rdi+R_PARENT]
	mov [rax+G_REGION], rcx
	test rcx, rcx
	jz rt_alloc_unhold
	ret

	.globl rt_region_suspend
rt_region_suspend:
	TLS_GET rcx, TLS_G
	mov rax, [rcx+G_REGION]
	test rax, rax
	jz 1f
	mov qword ptr [rcx+G_REGION], 0
	push rax
	call rt_alloc_unhold
	pop rax
1:
	ret

	.globl rt_region_resume
rt_region_resume:
	test rdi, rdi
	jz 1f
	TLS_GET rax, TLS_G
	mov [rax+G_REGION], rdi
	jmp rt_alloc_hold
1:
	ret

rt_alloc_hold:
	TLS_GET rcx, TLS_M
	mov rax, [rcx+M_ALLOC_END]
	test rax, rax
	jz 1f
	mov [rcx+M_ALLOC_HELD], rax
	mov qword ptr [rcx+M_ALLOC_END], 0
1:
	ret

rt_alloc_unhold:
	TLS_GET rcx, TLS_M
	mov rax, [rcx+M_ALLOC_HELD]
	test rax, rax
	jz 1f
	mov [rcx+M_ALLOC_END], rax
	mov qword ptr [rcx+M_ALLOC_HELD], 0
1:
	ret

	.globl rt_region_free
rt_region_free:
	ENTER_RT
	push rbx
	push r12
	push r13
	mov rbx, rdi
	call rt_heap_acquire
	mov rax, [rbx+R_INDEX]
	mov rcx, [rip+rt_region_count]
	dec rcx
	mov [rip+rt_region_count], rcx
	mov rdx, [rip+rt_region_list]
	mov rsi, [rdx+8*rcx]
	mov [rdx+8*rax], rsi
	mov [rsi+R_INDEX], rax
	mov qword ptr [rip+rt_heap_lock], 0
	mov rax, [rbx+R_SCAN_CHUNK]
	test rax, rax
	jz 1f
	mov rcx, [rbx+R_SCAN_BUMP]
	mov [rax+CH_USED], rcx
1:
	mov rax, [rbx+R_NOSCAN_CHUNK]
	mov rcx, [rbx+R_NOSCAN_BUMP]
	mov [rax+CH_USED], rcx
	mov r12, [rbx+R_CHUNKS]
2:
	test r12, r12
	jz 3f
	mov r13, [r12+CH_NEXT]
	mov rdi, r12
	call rt_region_release
	mov r12, r13
	jmp 2b
3:
	pop r13
	pop r12
	pop rbx
	ret

rt_region_release:
	push rbx
	push r12
	push r13
	mov rbx, rdi
	movzx r12d, byte ptr [rbx+CH_CLASS]
	mov edi, r12d
	call rt_region_keep_head
	test rcx, rcx
	jz 2f
	mov r13, rcx
	mov r8, [rcx]
	xor r9d, r9d
	test r8, r8
	jz 1f
	mov rdi, r8
	call rt_region_link
	mov r9, [rax+8]
	cmp r9, rdx
	jae 2f
1:
	mov rdi, rbx
	call rt_region_link
	mov [rax], r8
	inc r9
	mov [rax+8], r9
	mov [r13], rbx
	mov rdi, rbx
	mov ecx, r12d
	mov esi, 65536
	shl rsi, cl
	mov rdx, [rbx+CH_USED]
	call rt_region_stash
	jmp 3f
2:
	mov rdi, rbx
	mov ecx, r12d
	mov esi, 65536
	shl rsi, cl
	call rt_region_decommit
	call rt_heap_acquire
	mov rax, rbx
	sub rax, [rip+rt_region_start]
	shr rax, REGION_SHIFT
	mov rdx, r12
	shl rdx, 32
	or rax, rdx
	mov rcx, [rip+rt_rq_count]
	mov rdx, [rip+rt_rq]
	mov [rdx+8*rcx], rax
	inc rcx
	mov [rip+rt_rq_count], rcx
	mov qword ptr [rip+rt_heap_lock], 0
3:
	pop r13
	pop r12
	pop rbx
	ret

rt_spawn_region:
	test rsi, rsi
	jz rt_spawn_start
	mov rax, rsi
	sub rax, [rip+rt_region_start]
	shr rax, REGION_BITS
	jnz rt_spawn_start
	ENTER_RT
	push rdi
	push rsi
	mov rdi, [rsi-8]
	shr rdi, 3
	dec rdi
	push rdi
	TLS_GET rax, TLS_G
	mov rcx, [rax+G_REGION]
	push rcx
	mov qword ptr [rax+G_REGION], 0
	mov esi, 1
	call rt_alloc_large
	pop r9
	pop rcx
	pop rsi
	pop rdx
	TLS_GET r8, TLS_G
	mov [r8+G_REGION], r9
	mov rdi, rax
	shl rcx, 3
	rep movsb
	mov rsi, rax
	mov rdi, rdx
	jmp rt_spawn_start

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
