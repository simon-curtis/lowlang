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
rt_nl:
	.ascii "\n"
rt_panic_prefix:
	.ascii "panic: "
rt_error_prefix:
	.ascii "error: "
	MSG_BEGIN rt_msg_divzero
	.ascii "integer divide by zero"
	MSG_END
	MSG_BEGIN rt_msg_pins
	.ascii "too many pins"
	MSG_END
	MSG_BEGIN rt_msg_index
	.ascii "index out of range"
	MSG_END
	MSG_BEGIN rt_msg_oom
	.ascii "out of memory"
	MSG_END
	MSG_BEGIN rt_msg_region_full
	.ascii "region: address space exhausted"
	MSG_END

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
rt_mark_bits:
	.zero 8
rt_mark_base:
	.zero 8
rt_mark_ring:
	.zero 256
rt_live_words:
	.zero 8
rt_alloc_since:
	.zero 8
rt_gc_threshold:
	.zero 8
rt_dirty:
	.zero 8
rt_bin_bits:
	.zero 32
rt_bins:
	.zero 2048
rt_pin_count:
	.zero 8
rt_pins:
	.zero 524288
rt_free_small:
	.zero 512
	.globl rt_hash_seed
rt_hash_seed:
	.zero 16
rt_arg_count:
	.zero 8
rt_arg_values:
	.zero 8
rt_env_values:
	.zero 8

	.text

	.macro MARK_CANDIDATE
	mov r8, [rip+rt_heap_start]
	cmp rdi, r8
	jb .Lmark_done_\@
	lea rax, [r8+8]
	cmp rdi, rax
	jb .Lmark_search_\@
	cmp rdi, [rip+rt_heap_bump]
	jae .Lmark_far_\@
	lea rax, [rdi-8]
	sub rax, r8
	shr rax, 3
	mov rcx, rax
	shr rcx, 6
	mov rdx, [rip+rt_bitmap]
	mov rdx, [rdx+8*rcx]
	bt rdx, rax
	jnc .Lmark_search_\@
	jmp .Lmark_resolved_\@
.Lmark_far_\@:
	mov rax, rdi
	sub rax, [rip+rt_region_start]
	mov rcx, rax
	shr rcx, REGION_BITS
	jnz .Lmark_done_\@
	shr rax, REGION_SHIFT
	mov rcx, rax
	shr rcx, 6
	mov rdx, [rip+rt_region_hits]
	lea rdx, [rdx+8*rcx]
	mov rcx, [rdx]
	bt rcx, rax
	jc .Lmark_done_\@
	bts rcx, rax
	mov [rdx], rcx
	jmp .Lmark_done_\@
.Lmark_search_\@:
	call rt_find
	test rax, rax
	jz .Lmark_done_\@
	sub rax, [rip+rt_heap_start]
	shr rax, 3
.Lmark_resolved_\@:
	mov rcx, rax
	shr rcx, 6
	mov r9, [rip+rt_mark_bits]
	lea r9, [r9+8*rcx]
	mov rdx, [r9]
	bt rdx, rax
	jc .Lmark_done_\@
	mov r8, [rip+rt_heap_start]
	lea rcx, [r8+8*rax]
	mov rsi, [rcx]
	test sil, 2
	jnz .Lmark_done_\@
	bts rdx, rax
	mov [r9], rdx
	mov r10, rsi
	shr r10, 3
	add [rip+rt_live_words], r10
	test sil, 4
	jnz .Lmark_done_\@
	mov [r15], rcx
	add r15, 8
.Lmark_done_\@:
	.endm

	.globl rt_str_eq
rt_str_eq:
	xor eax, eax
	cmp rsi, rcx
	jne 9f
	cmp rdi, rdx
	je 8f
	cmp rsi, 8
	jb 3f
1:
	cmp rsi, 8
	jbe 2f
	mov r10, [rdi]
	cmp r10, [rdx]
	jne 9f
	add rdi, 8
	add rdx, 8
	sub rsi, 8
	jmp 1b
2:
	mov r10, [rdi+rsi-8]
	cmp r10, [rdx+rsi-8]
	sete al
	ret
3:
	cmp rsi, 4
	jb 4f
	mov r10d, [rdi]
	cmp r10d, [rdx]
	jne 9f
	mov r10d, [rdi+rsi-4]
	cmp r10d, [rdx+rsi-4]
	sete al
	ret
4:
	test rsi, rsi
	jz 8f
	movzx r10d, byte ptr [rdi]
	cmp r10b, [rdx]
	jne 9f
	movzx r10d, byte ptr [rdi+rsi-1]
	cmp r10b, [rdx+rsi-1]
	jne 9f
	shr rsi, 1
	movzx r10d, byte ptr [rdi+rsi]
	cmp r10b, [rdx+rsi]
	sete al
	ret
8:
	mov eax, 1
9:
	ret

	.globl rt_memmove
rt_memmove:
	cmp rdx, 16
	ja 5f
	cmp rdx, 8
	jae 1f
	cmp rdx, 4
	jae 2f
	test rdx, rdx
	jz 9f
	movzx eax, byte ptr [rsi]
	movzx ecx, byte ptr [rsi+rdx-1]
	mov r8, rdx
	shr r8, 1
	movzx r9d, byte ptr [rsi+r8]
	mov [rdi], al
	mov [rdi+rdx-1], cl
	mov [rdi+r8], r9b
9:
	ret
2:
	mov eax, [rsi]
	mov ecx, [rsi+rdx-4]
	mov [rdi], eax
	mov [rdi+rdx-4], ecx
	ret
1:
	mov rax, [rsi]
	mov rcx, [rsi+rdx-8]
	mov [rdi], rax
	mov [rdi+rdx-8], rcx
	ret
5:
	cmp rdx, 32
	ja 6f
	movups xmm0, [rsi]
	movups xmm1, [rsi+rdx-16]
	movups [rdi], xmm0
	movups [rdi+rdx-16], xmm1
	ret
6:
	cmp rdx, 64
	ja 7f
	movups xmm0, [rsi]
	movups xmm1, [rsi+16]
	movups xmm2, [rsi+rdx-32]
	movups xmm3, [rsi+rdx-16]
	movups [rdi], xmm0
	movups [rdi+16], xmm1
	movups [rdi+rdx-32], xmm2
	movups [rdi+rdx-16], xmm3
	ret
7:
	mov rcx, rdx
	cmp rdi, rsi
	jbe 8f
	lea rax, [rsi+rdx]
	cmp rdi, rax
	jae 8f
	lea rsi, [rsi+rdx-1]
	lea rdi, [rdi+rdx-1]
	std
	rep movsb
	cld
	ret
8:
	rep movsb
	ret

	.macro GROW scan
	push rcx
	push rsi
	push rdi
	push r8
	push r9
	TLS_GET rcx, TLS_G
	mov r11, [rsp+40]
	mov [rcx+G_PC], r11
	mov [rcx+G_USERSP], rsp
	mov [rcx+G_FP], rbp
	push rax
	push rdx
	push r10
	lea rdi, [r10+7]
	shr rdi, 3
	mov esi, \scan
	cmp rdi, 1
	adc rdi, 0
	cmp rdi, ALLOC_SMALL
	ja 1f
	call rt_alloc_small
	jmp 2f
1:
	cmp qword ptr [rcx+G_REGION], 0
	jne 3f
	call rt_heap_acquire
	call rt_alloc_body
	mov qword ptr [rip+rt_heap_lock], 0
	call rt_zero_payload
	jmp 2f
3:
	mov rax, rcx
	call rt_region_alloc
2:
	mov rdi, rax
	mov r11, rax
	mov rsi, [rsp+16]
	mov rdx, [rsp+8]
	call rt_memmove
	mov rax, r11
	add rsp, 24
	pop r9
	pop r8
	pop rdi
	pop rsi
	pop rcx
	ret
	.endm

	.globl rt_grow_scan
rt_grow_scan:
	GROW 1

	.globl rt_grow_noscan
rt_grow_noscan:
	GROW 0

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
	call rt_gc_env
	mov rsi, 85899345920
	call rt_mmap
	mov [rip+rt_heap_start], rax
	mov [rip+rt_heap_bump], rax
	mov rcx, 17179869184
	add rcx, rax
	mov [rip+rt_heap_end], rcx
	mov [rip+rt_region_start], rcx
	mov rsi, 268435456
	call rt_mmap_sparse
	mov [rip+rt_bitmap], rax
	mov rsi, 268435456
	call rt_mmap_sparse
	mov [rip+rt_mark_bits], rax
	mov rsi, 4194304
	call rt_mmap_sparse
	mov [rip+rt_summary1], rax
	mov rsi, 65536
	call rt_mmap
	mov [rip+rt_summary2], rax
	mov rsi, 8589934592
	call rt_mmap_sparse
	mov [rip+rt_mark_base], rax
	mov rsi, 524288
	call rt_mmap
	mov [rip+rt_dirty], rax
	call rt_region_init
	xor eax, eax
	call rt_gc_set_threshold
	ret

rt_set_start:
	mov rax, rdi
	sub rax, [rip+rt_heap_start]
	shr rax, 3
	mov rcx, rax
	shr rcx, 6
	mov rdx, [rip+rt_bitmap]
	lea rcx, [rdx+8*rcx]
	xor edx, edx
	bts rdx, rax
	lock or [rcx], rdx
	shr rax, 6
	jmp rt_set_summaries

rt_set_summaries:
	mov rcx, rax
	shr rcx, 6
	mov rdx, [rip+rt_summary1]
	lea rcx, [rdx+8*rcx]
	mov rdx, [rcx]
	bt rdx, rax
	jc 1f
	xor edx, edx
	bts rdx, rax
	lock or [rcx], rdx
1:
	shr rax, 6
	mov rcx, rax
	shr rcx, 6
	mov rdx, [rip+rt_summary2]
	lea rcx, [rdx+8*rcx]
	mov rdx, [rcx]
	bt rdx, rax
	jc 2f
	xor edx, edx
	bts rdx, rax
	lock or [rcx], rdx
2:
	ret

rt_clear_start:
	mov rax, rdi
	sub rax, [rip+rt_heap_start]
	shr rax, 3
	mov rdx, rax
	shr rdx, 6
	mov rcx, [rip+rt_bitmap]
	lea rcx, [rcx+8*rdx]
	mov r8, [rcx]
	btr r8, rax
	mov [rcx], r8
	test r8, r8
	jnz 1f
	jmp rt_clear_summaries
1:
	ret

rt_clear_summaries:
	mov rax, rdx
	shr rdx, 6
	mov rcx, [rip+rt_summary1]
	lea rcx, [rcx+8*rdx]
	mov r8, [rcx]
	btr r8, rax
	mov [rcx], r8
	test r8, r8
	jnz 1f
	mov rax, rdx
	shr rdx, 6
	mov rcx, [rip+rt_summary2]
	lea rcx, [rcx+8*rdx]
	mov r8, [rcx]
	btr r8, rax
	mov [rcx], r8
1:
	ret

rt_free_chunk:
	call rt_link_chunk
	jmp rt_set_start

rt_link_chunk:
	mov rax, rsi
	shl rax, 3
	or rax, 2
	mov [rdi], rax
	mov r8, [rip+rt_heap_start]
	mov r9, [rip+rt_dirty]
	mov rax, rdi
	sub rax, r8
	shr rax, PAGE_SHIFT
	lock bts qword ptr [r9], rax
	lea rax, [rdi+15]
	sub rax, r8
	shr rax, PAGE_SHIFT
	lock bts qword ptr [r9], rax
	cmp rsi, BIN_MIN
	jb 4f
	xor r10d, r10d
	cmp rsi, RELEASE_MIN
	jb 2f
	lea rax, [rdi+16+4095]
	sub rax, r8
	shr rax, PAGE_SHIFT
	lea rcx, [rdi+8*rsi]
	sub rcx, r8
	shr rcx, PAGE_SHIFT
	dec rcx
	push rdi
	push rsi
	mov rdi, rax
	mov rsi, rcx
	xor edx, edx
	call rt_page_scan
	cmp rax, rsi
	pop rsi
	pop rdi
	jbe 2f
	mov r10d, BIN_CLEAN
2:
	call rt_bin_of
	add rax, r10
	lea rcx, [rip+rt_bins]
	mov rdx, [rcx+8*rax]
	mov [rdi+8], rdx
	mov [rcx+8*rax], rdi
	lea rcx, [rip+rt_bin_bits]
	bts qword ptr [rcx], rax
	ret
4:
	lea rcx, [rip+rt_free_small]
	lea rcx, [rcx+8*rsi]
	mov rax, [rcx]
	mov [rdi+8], rax
	mov [rcx], rdi
	ret

rt_bin_of:
	bsr rdx, rsi
	lea ecx, [rdx-2]
	mov rax, rsi
	shr rax, cl
	and eax, 3
	sub edx, 6
	lea rax, [rax+4*rdx]
	ret

rt_bits_next:
	cmp rdi, 64
	jae 2f
	mov ecx, edi
	mov rax, -1
	shl rax, cl
	and rax, [rsi]
	jz 1f
	bsf rax, rax
	ret
1:
	mov rax, [rsi+8]
	test rax, rax
	jz 3f
	bsf rax, rax
	add rax, 64
	ret
2:
	cmp rdi, 128
	jae 3f
	lea ecx, [rdi-64]
	mov rax, -1
	shl rax, cl
	and rax, [rsi+8]
	jz 3f
	bsf rax, rax
	add rax, 64
	ret
3:
	mov rax, -1
	ret

rt_bin_scan:
	lea r8, [rip+rt_bins]
	lea r8, [r8+8*rdi]
	mov r10, r8
	mov r9d, BIN_SCAN
1:
	mov rax, [r8]
	test rax, rax
	jz 9f
	mov rdx, [rax]
	shr rdx, 3
	cmp rdx, rsi
	jae 2f
	lea r8, [rax+8]
	dec r9d
	jnz 1b
9:
	xor eax, eax
	ret
2:
	mov rdx, [rax+8]
	mov [r8], rdx
	cmp qword ptr [r10], 0
	jne 3f
	lea rcx, [rip+rt_bin_bits]
	btr qword ptr [rcx], rdi
3:
	ret

rt_bin_take:
	cmp rdi, BIN_MIN
	jae 1f
	mov edi, BIN_MIN
1:
	push rbx
	push r12
	mov r12, rdi
	mov rsi, rdi
	call rt_bin_of
	mov rbx, rax
	mov rax, 1
	shl rax, cl
	dec rax
	test r12, rax
	jz 3f
	mov rdi, rbx
	mov rsi, r12
	call rt_bin_scan
	test rax, rax
	jnz 8f
	lea rdi, [rbx+BIN_CLEAN]
	mov rsi, r12
	call rt_bin_scan
	test rax, rax
	jnz 8f
	inc rbx
3:
	mov rdi, rbx
	lea rsi, [rip+rt_bin_bits]
	call rt_bits_next
	mov r12, rax
	mov rdi, rbx
	lea rsi, [rip+rt_bin_bits+16]
	call rt_bits_next
	cmp r12, -1
	je 4f
	cmp rax, -1
	je 6f
	cmp rax, r12
	jae 6f
	lea r12, [rax+BIN_CLEAN]
	jmp 6f
4:
	cmp rax, -1
	je 7f
	lea r12, [rax+BIN_CLEAN]
6:
	lea rcx, [rip+rt_bins]
	mov rax, [rcx+8*r12]
	mov rdx, [rax+8]
	mov [rcx+8*r12], rdx
	test rdx, rdx
	jnz 8f
	lea rcx, [rip+rt_bin_bits]
	btr qword ptr [rcx], r12
	jmp 8f
7:
	xor eax, eax
8:
	pop r12
	pop rbx
	ret

rt_page_scan:
	mov r9, [rip+rt_dirty]
	mov r8, rdi
1:
	cmp r8, rsi
	ja 3f
	mov rcx, r8
	shr rcx, 6
	mov rax, [r9+8*rcx]
	xor rax, rdx
	mov ecx, r8d
	shr rax, cl
	test rax, rax
	jnz 2f
	or r8, 63
	inc r8
	jmp 1b
2:
	bsf rax, rax
	add rax, r8
	cmp rax, rsi
	ja 3f
	ret
3:
	lea rax, [rsi+1]
	ret

rt_page_set:
	mov r10, -1
	jmp rt_page_write

rt_page_clear:
	xor r10d, r10d

rt_page_write:
	mov r8, [rip+rt_dirty]
1:
	mov rdx, rdi
	shr rdx, 6
	mov rax, -1
	mov ecx, edi
	shl rax, cl
	mov rcx, rsi
	shr rcx, 6
	cmp rcx, rdx
	jne 2f
	mov ecx, esi
	not ecx
	and ecx, 63
	mov r9, -1
	shr r9, cl
	and rax, r9
2:
	test r10, r10
	jz 3f
	lock or [r8+8*rdx], rax
	jmp 4f
3:
	not rax
	lock and [r8+8*rdx], rax
4:
	lea rdi, [rdx+1]
	shl rdi, 6
	cmp rdi, rsi
	jbe 1b
	ret

rt_zero_range:
	push rbx
	push r12
	push r13
	push r14
	push r15
	mov rbx, rdi
	lea r12, [rdi+8*rsi]
	mov r13, [rip+rt_heap_start]
	mov r14, rdi
	sub r14, r13
	shr r14, PAGE_SHIFT
	lea r15, [r12-1]
	sub r15, r13
	shr r15, PAGE_SHIFT
1:
	cmp r14, r15
	ja 9f
	mov rdi, r14
	mov rsi, r15
	xor edx, edx
	call rt_page_scan
	cmp rax, r15
	ja 9f
	mov r14, rax
	mov rdi, rax
	mov rsi, r15
	mov rdx, -1
	call rt_page_scan
	mov rcx, r14
	shl rcx, PAGE_SHIFT
	add rcx, r13
	cmp rcx, rbx
	cmovb rcx, rbx
	mov rdx, rax
	shl rdx, PAGE_SHIFT
	add rdx, r13
	cmp rdx, r12
	cmova rdx, r12
	mov r14, rax
	mov rdi, rcx
	mov rcx, rdx
	sub rcx, rdi
	shr rcx, 3
	xor eax, eax
	rep stosq
	jmp 1b
9:
	mov rdi, rbx
	sub rdi, r13
	shr rdi, PAGE_SHIFT
	mov rsi, r15
	call rt_page_set
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	ret

rt_next_word:
	mov r11, [rip+rt_heap_bump]
	sub r11, [rip+rt_heap_start]
	add r11, 511
	shr r11, 9
	cmp rdi, r11
	jae 7f
	add r11, 4095
	shr r11, 12
	mov rdx, rdi
	mov rsi, rdx
	shr rsi, 6
	mov r9, [rip+rt_summary1]
	mov rax, [r9+8*rsi]
	mov ecx, edx
	shr rax, cl
	test rax, rax
	jz 1f
	bsf rax, rax
	add rax, rdx
	ret
1:
	inc rsi
	mov r10, [rip+rt_summary2]
	mov rdx, rsi
	shr rdx, 6
	cmp rdx, r11
	jae 7f
	mov rax, [r10+8*rdx]
	mov ecx, esi
	shr rax, cl
	test rax, rax
	jz 3f
	bsf rax, rax
	add rax, rsi
	jmp 5f
3:
	inc rdx
4:
	cmp rdx, r11
	jae 7f
	mov rax, [r10+8*rdx]
	test rax, rax
	jnz 6f
	inc rdx
	jmp 4b
6:
	bsf rax, rax
	shl rdx, 6
	add rax, rdx
5:
	mov rdx, [r9+8*rax]
	bsf rdx, rdx
	shl rax, 6
	add rax, rdx
	ret
7:
	mov rax, -1
	ret

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
	cmp qword ptr [rax+G_REGION], 0
	jne rt_region_alloc
rt_alloc_large:
	push rsi
	call rt_heap_acquire
	pop rsi
	call rt_alloc_body
	mov qword ptr [rip+rt_heap_lock], 0
	jmp rt_zero_payload

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
	mov rdx, rcx
	shr rdx, 6
	mov r10, [rip+rt_bitmap]
	lea rdx, [r10+8*rdx]
	xor r10d, r10d
	bts r10, rcx
	cmp rax, [r8+M_ALLOC_LO]
	jb 4f
	cmp rax, [r8+M_ALLOC_HI]
	jae 4f
	or [rdx], r10
	jmp 6f
4:
	lock or [rdx], r10
6:
	shr rcx, 6
	cmp rcx, [r8+M_ALLOC_SUMMARY]
	je 7f
	mov [r8+M_ALLOC_SUMMARY], rcx
	mov rax, rcx
	call rt_set_summaries
7:
	lea rax, [r9+8]
	ret
5:
	TLS_GET rax, TLS_G
	cmp qword ptr [rax+G_REGION], 0
	jne rt_region_alloc
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
	inc qword ptr [rbx+M_SLOWS]
	inc qword ptr [rbx+M_REFILLS]
	mov rdi, [rbx+M_ALLOC_CUR]
	mov rsi, [rbx+M_ALLOC_END]
	mov qword ptr [rbx+M_ALLOC_CUR], 0
	mov qword ptr [rbx+M_ALLOC_END], 0
	call rt_alloc_retire
	mov rax, [rip+rt_alloc_since]
	cmp rax, [rip+rt_gc_threshold]
	jb 1f
	call rt_gc
1:
	mov rdi, r12
	call rt_bin_take
	test rax, rax
	jz 5f
3:
	mov r13, [rax]
	shr r13, 3
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
	add [rip+rt_alloc_since], r13
	mov rdx, rax
	mov [rbx+M_ALLOC_CUR], rdx
	mov qword ptr [rbx+M_ALLOC_SUMMARY], -1
	lea rax, [rdx+8*r13]
	mov [rbx+M_ALLOC_END], rax
	and rax, -512
	mov [rbx+M_ALLOC_HI], rax
	add rdx, 511
	and rdx, -512
	mov [rbx+M_ALLOC_LO], rdx
	mov qword ptr [rip+rt_heap_lock], 0
	mov rdi, [rbx+M_ALLOC_CUR]
	mov rsi, r13
	call rt_zero_range
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
	test rsi, rsi
	jnz 3f
	mov rsi, [rbx+M_ALLOC_HELD]
3:
	mov qword ptr [rbx+M_ALLOC_CUR], 0
	mov qword ptr [rbx+M_ALLOC_END], 0
	mov qword ptr [rbx+M_ALLOC_HELD], 0
	call rt_alloc_retire
	jmp 1b
2:
	pop r12
	pop rbx
	ret

rt_alloc_body:
	push rsi
	inc qword ptr [rip+rt_stat_slow]
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
	push rsi
	mov rdi, rsi
	call rt_bin_take
	pop rsi
	test rax, rax
	jz 5f
	mov rdx, [rax]
	shr rdx, 3
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
	push rsi
	call rt_heap_release
	pop rsi
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
	shr rdx, 3
	lea rcx, [rdx-1]
	add rax, 8
	ret

rt_zero_payload:
	push rax
	mov rdi, rax
	mov rsi, rcx
	call rt_zero_range
	pop rax
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
	MARK_CANDIDATE
	ret

rt_gc:
	push rbx
	push r12
	push r13
	push r14
	push r15
	mov edi, TICK_ENTER
	call rt_gc_stamp
	TLS_GET rax, TLS_G
	mov [rax+G_SCANLO], rsp
	call rt_world_stop
	mov edi, TICK_STOPPED
	call rt_gc_stamp
	call rt_alloc_retire_all
	mov qword ptr [rip+rt_live_words], 0
	call rt_region_hits_clear
	mov r15, [rip+rt_mark_base]
	lea rdi, [rip+rt_pins]
	mov rsi, [rip+rt_pin_count]
	lea rsi, [rdi+8*rsi]
	call rt_scan_range
	lea rdi, [rip+rt_cells]
	lea rsi, [rip+rt_cells_end]
	call rt_scan_range
	call rt_scan_regions
	xor r12d, r12d
1:
	cmp r12, [rip+rt_ngs]
	jae 5f
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
5:
	xor ebx, ebx
6:
	cmp r15, [rip+rt_mark_base]
	jbe 9f
	sub r15, 8
	mov r12, [r15]
	mov r13, [r12]
	shr r13, 3
	mov r14d, 1
7:
	cmp r14, r13
	jae 6b
	mov rdi, [r12+8*r14]
	inc r14
	cmp rdi, [rip+rt_heap_start]
	jb 7b
	prefetcht0 [rdi-8]
	mov rax, rbx
	and eax, 31
	lea rcx, [rip+rt_mark_ring]
	mov rdx, [rcx+8*rax]
	mov [rcx+8*rax], rdi
	mov rdi, rdx
	inc rbx
	test rdi, rdi
	jz 7b
	MARK_CANDIDATE
	jmp 7b
9:
	xor r13d, r13d
10:
	lea rcx, [rip+rt_mark_ring]
	mov rdi, [rcx+8*r13]
	mov qword ptr [rcx+8*r13], 0
	inc r13
	test rdi, rdi
	jz 11f
	MARK_CANDIDATE
11:
	cmp r13, 32
	jb 10b
	cmp r15, [rip+rt_mark_base]
	ja 6b
	mov edi, TICK_MARKED
	call rt_gc_stamp
	call rt_region_reclaim
	call rt_sweep
	mov edi, TICK_SWEPT
	call rt_gc_stamp
	call rt_world_start
	mov edi, TICK_RESUMED
	call rt_gc_stamp
	inc qword ptr [rip+rt_gc_count]
	cmp byte ptr [rip+rt_gc_trace], 0
	je 12f
	call rt_gc_stats_collect
	call rt_gc_trace_print
12:
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
	add rbx, 8
	MARK_CANDIDATE
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
	lea rdi, [rip+rt_bin_bits]
	mov ecx, 260
	rep stosq
	push rbp
	xor r13d, r13d
	xor r14d, r14d
1:
	mov rdi, r14
	call rt_next_word
	cmp rax, -1
	je 5f
	mov r14, rax
	mov rax, [rip+rt_bitmap]
	mov rdx, [rax+8*r14]
	mov rax, [rip+rt_mark_bits]
	mov rbx, [rax+8*r14]
	and rbx, rdx
	mov r12, rbx
	not r12
	and r12, rdx
	mov rbp, -1
2:
	test r13, r13
	jnz 4f
	mov rax, r12
	and rax, rbp
	jz 7f
	bsf rcx, rax
	mov rdx, r14
	shl rdx, 6
	add rdx, rcx
	mov r13, [rip+rt_heap_start]
	lea r13, [r13+8*rdx]
	bts rbx, rcx
	mov rax, -2
	shl rax, cl
	and rbp, rax
	jmp 2b
4:
	mov rax, rbx
	and rax, rbp
	jz 7f
	bsf rcx, rax
	mov rdx, -2
	shl rdx, cl
	and rbp, rdx
	mov rdx, r14
	shl rdx, 6
	add rdx, rcx
	mov rsi, [rip+rt_heap_start]
	lea rsi, [rsi+8*rdx]
	mov rdi, r13
	sub rsi, r13
	shr rsi, 3
	call rt_link_chunk
	xor r13d, r13d
	jmp 2b
7:
	mov rax, [rip+rt_mark_bits]
	cmp qword ptr [rax+8*r14], 0
	je 6f
	mov qword ptr [rax+8*r14], 0
6:
	mov rax, [rip+rt_bitmap]
	mov [rax+8*r14], rbx
	test rbx, rbx
	jnz 8f
	mov rdx, r14
	call rt_clear_summaries
8:
	inc r14
	jmp 1b
5:
	test r13, r13
	jz 3f
	mov [rip+rt_heap_bump], r13
	mov rdi, r13
	call rt_clear_start
3:
	pop rbp
	mov rax, [rip+rt_live_words]
	call rt_gc_set_threshold
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

rt_seed_mix:
	mov rdx, 0x9E3779B97F4A7C15
	add rax, rdx
	mov rdx, rax
	shr rdx, 30
	xor rax, rdx
	mov rdx, 0xBF58476D1CE4E5B9
	imul rax, rdx
	mov rdx, rax
	shr rdx, 27
	xor rax, rdx
	mov rdx, 0x94D049BB133111EB
	imul rax, rdx
	mov rdx, rax
	shr rdx, 31
	xor rax, rdx
	ret

rt_seed_fallback:
	push rbx
	rdtsc
	shl rdx, 32
	or rax, rdx
	mov r8, rax
	mov eax, 1
	xor ecx, ecx
	cpuid
	mov r9d, ecx
	rdtsc
	shl rdx, 32
	or rax, rdx
	mov r10, rax
	mov r11, rsp
	bt r9d, 30
	jnc 4f
	mov ecx, 10
1:
	rdrand rax
	jc 2f
	dec ecx
	jnz 1b
	jmp 4f
2:
	xor r8, rax
	mov ecx, 10
3:
	rdrand rax
	jc 5f
	dec ecx
	jnz 3b
	jmp 4f
5:
	xor r10, rax
4:
	mov rax, r8
	xor rax, r11
	call rt_seed_mix
	mov r8, rax
	mov rax, r10
	ror r11, 29
	xor rax, r11
	call rt_seed_mix
	mov rdx, rax
	mov rax, r8
	pop rbx
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
	call rt_seed_init
	mov rdi, r12
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
