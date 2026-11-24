.intel_syntax noprefix
.text
# three operand forms
vaddps zmm0, zmm1, zmm2
vaddps zmm31, zmm30, zmm29
vaddps ymm8, ymm9, ymm16
vaddps xmm16, xmm17, xmm18
vaddps zmm3{k1}{z}, zmm20, zmm25
vaddps ymm3{k7}, ymm4, ymm5
vaddps zmm1, zmm2, zmmword ptr [rax+64]
vaddps ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vaddps xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vaddps zmm1, zmm2, dword ptr [rax+4]{1to16}
vaddps ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vaddps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vaddpd zmm0, zmm1, zmm2
vaddpd zmm31, zmm30, zmm29
vaddpd ymm8, ymm9, ymm16
vaddpd xmm16, xmm17, xmm18
vaddpd zmm3{k1}{z}, zmm20, zmm25
vaddpd ymm3{k7}, ymm4, ymm5
vaddpd zmm1, zmm2, zmmword ptr [rax+64]
vaddpd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vaddpd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vaddpd zmm1, zmm2, qword ptr [rax+8]{1to8}
vaddpd ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vaddpd xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vmulps zmm0, zmm1, zmm2
vmulps zmm31, zmm30, zmm29
vmulps ymm8, ymm9, ymm16
vmulps xmm16, xmm17, xmm18
vmulps zmm3{k1}{z}, zmm20, zmm25
vmulps ymm3{k7}, ymm4, ymm5
vmulps zmm1, zmm2, zmmword ptr [rax+64]
vmulps ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vmulps xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vmulps zmm1, zmm2, dword ptr [rax+4]{1to16}
vmulps ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vmulps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vmulpd zmm0, zmm1, zmm2
vmulpd zmm31, zmm30, zmm29
vmulpd ymm8, ymm9, ymm16
vmulpd xmm16, xmm17, xmm18
vmulpd zmm3{k1}{z}, zmm20, zmm25
vmulpd ymm3{k7}, ymm4, ymm5
vmulpd zmm1, zmm2, zmmword ptr [rax+64]
vmulpd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vmulpd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vmulpd zmm1, zmm2, qword ptr [rax+8]{1to8}
vmulpd ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vmulpd xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vsubps zmm0, zmm1, zmm2
vsubps zmm31, zmm30, zmm29
vsubps ymm8, ymm9, ymm16
vsubps xmm16, xmm17, xmm18
vsubps zmm3{k1}{z}, zmm20, zmm25
vsubps ymm3{k7}, ymm4, ymm5
vsubps zmm1, zmm2, zmmword ptr [rax+64]
vsubps ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vsubps xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vsubps zmm1, zmm2, dword ptr [rax+4]{1to16}
vsubps ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vsubps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vsubpd zmm0, zmm1, zmm2
vsubpd zmm31, zmm30, zmm29
vsubpd ymm8, ymm9, ymm16
vsubpd xmm16, xmm17, xmm18
vsubpd zmm3{k1}{z}, zmm20, zmm25
vsubpd ymm3{k7}, ymm4, ymm5
vsubpd zmm1, zmm2, zmmword ptr [rax+64]
vsubpd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vsubpd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vsubpd zmm1, zmm2, qword ptr [rax+8]{1to8}
vsubpd ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vsubpd xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vminps zmm0, zmm1, zmm2
vminps zmm31, zmm30, zmm29
vminps ymm8, ymm9, ymm16
vminps xmm16, xmm17, xmm18
vminps zmm3{k1}{z}, zmm20, zmm25
vminps ymm3{k7}, ymm4, ymm5
vminps zmm1, zmm2, zmmword ptr [rax+64]
vminps ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vminps xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vminps zmm1, zmm2, dword ptr [rax+4]{1to16}
vminps ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vminps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vminpd zmm0, zmm1, zmm2
vminpd zmm31, zmm30, zmm29
vminpd ymm8, ymm9, ymm16
vminpd xmm16, xmm17, xmm18
vminpd zmm3{k1}{z}, zmm20, zmm25
vminpd ymm3{k7}, ymm4, ymm5
vminpd zmm1, zmm2, zmmword ptr [rax+64]
vminpd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vminpd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vminpd zmm1, zmm2, qword ptr [rax+8]{1to8}
vminpd ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vminpd xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vdivps zmm0, zmm1, zmm2
vdivps zmm31, zmm30, zmm29
vdivps ymm8, ymm9, ymm16
vdivps xmm16, xmm17, xmm18
vdivps zmm3{k1}{z}, zmm20, zmm25
vdivps ymm3{k7}, ymm4, ymm5
vdivps zmm1, zmm2, zmmword ptr [rax+64]
vdivps ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vdivps xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vdivps zmm1, zmm2, dword ptr [rax+4]{1to16}
vdivps ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vdivps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vdivpd zmm0, zmm1, zmm2
vdivpd zmm31, zmm30, zmm29
vdivpd ymm8, ymm9, ymm16
vdivpd xmm16, xmm17, xmm18
vdivpd zmm3{k1}{z}, zmm20, zmm25
vdivpd ymm3{k7}, ymm4, ymm5
vdivpd zmm1, zmm2, zmmword ptr [rax+64]
vdivpd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vdivpd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vdivpd zmm1, zmm2, qword ptr [rax+8]{1to8}
vdivpd ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vdivpd xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vmaxps zmm0, zmm1, zmm2
vmaxps zmm31, zmm30, zmm29
vmaxps ymm8, ymm9, ymm16
vmaxps xmm16, xmm17, xmm18
vmaxps zmm3{k1}{z}, zmm20, zmm25
vmaxps ymm3{k7}, ymm4, ymm5
vmaxps zmm1, zmm2, zmmword ptr [rax+64]
vmaxps ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vmaxps xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vmaxps zmm1, zmm2, dword ptr [rax+4]{1to16}
vmaxps ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vmaxps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vmaxpd zmm0, zmm1, zmm2
vmaxpd zmm31, zmm30, zmm29
vmaxpd ymm8, ymm9, ymm16
vmaxpd xmm16, xmm17, xmm18
vmaxpd zmm3{k1}{z}, zmm20, zmm25
vmaxpd ymm3{k7}, ymm4, ymm5
vmaxpd zmm1, zmm2, zmmword ptr [rax+64]
vmaxpd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vmaxpd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vmaxpd zmm1, zmm2, qword ptr [rax+8]{1to8}
vmaxpd ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vmaxpd xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpaddb zmm0, zmm1, zmm2
vpaddb zmm31, zmm30, zmm29
vpaddb ymm8, ymm9, ymm16
vpaddb xmm16, xmm17, xmm18
vpaddb zmm3{k1}{z}, zmm20, zmm25
vpaddb ymm3{k7}, ymm4, ymm5
vpaddb zmm1, zmm2, zmmword ptr [rax+64]
vpaddb ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpaddb xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpaddw zmm0, zmm1, zmm2
vpaddw zmm31, zmm30, zmm29
vpaddw ymm8, ymm9, ymm16
vpaddw xmm16, xmm17, xmm18
vpaddw zmm3{k1}{z}, zmm20, zmm25
vpaddw ymm3{k7}, ymm4, ymm5
vpaddw zmm1, zmm2, zmmword ptr [rax+64]
vpaddw ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpaddw xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpaddd zmm0, zmm1, zmm2
vpaddd zmm31, zmm30, zmm29
vpaddd ymm8, ymm9, ymm16
vpaddd xmm16, xmm17, xmm18
vpaddd zmm3{k1}{z}, zmm20, zmm25
vpaddd ymm3{k7}, ymm4, ymm5
vpaddd zmm1, zmm2, zmmword ptr [rax+64]
vpaddd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpaddd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpaddd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpaddd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpaddd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpaddq zmm0, zmm1, zmm2
vpaddq zmm31, zmm30, zmm29
vpaddq ymm8, ymm9, ymm16
vpaddq xmm16, xmm17, xmm18
vpaddq zmm3{k1}{z}, zmm20, zmm25
vpaddq ymm3{k7}, ymm4, ymm5
vpaddq zmm1, zmm2, zmmword ptr [rax+64]
vpaddq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpaddq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpaddq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpaddq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpaddq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpsubb zmm0, zmm1, zmm2
vpsubb zmm31, zmm30, zmm29
vpsubb ymm8, ymm9, ymm16
vpsubb xmm16, xmm17, xmm18
vpsubb zmm3{k1}{z}, zmm20, zmm25
vpsubb ymm3{k7}, ymm4, ymm5
vpsubb zmm1, zmm2, zmmword ptr [rax+64]
vpsubb ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpsubb xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpsubw zmm0, zmm1, zmm2
vpsubw zmm31, zmm30, zmm29
vpsubw ymm8, ymm9, ymm16
vpsubw xmm16, xmm17, xmm18
vpsubw zmm3{k1}{z}, zmm20, zmm25
vpsubw ymm3{k7}, ymm4, ymm5
vpsubw zmm1, zmm2, zmmword ptr [rax+64]
vpsubw ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpsubw xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpsubd zmm0, zmm1, zmm2
vpsubd zmm31, zmm30, zmm29
vpsubd ymm8, ymm9, ymm16
vpsubd xmm16, xmm17, xmm18
vpsubd zmm3{k1}{z}, zmm20, zmm25
vpsubd ymm3{k7}, ymm4, ymm5
vpsubd zmm1, zmm2, zmmword ptr [rax+64]
vpsubd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpsubd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpsubd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpsubd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpsubd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpsubq zmm0, zmm1, zmm2
vpsubq zmm31, zmm30, zmm29
vpsubq ymm8, ymm9, ymm16
vpsubq xmm16, xmm17, xmm18
vpsubq zmm3{k1}{z}, zmm20, zmm25
vpsubq ymm3{k7}, ymm4, ymm5
vpsubq zmm1, zmm2, zmmword ptr [rax+64]
vpsubq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpsubq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpsubq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpsubq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpsubq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpmullw zmm0, zmm1, zmm2
vpmullw zmm31, zmm30, zmm29
vpmullw ymm8, ymm9, ymm16
vpmullw xmm16, xmm17, xmm18
vpmullw zmm3{k1}{z}, zmm20, zmm25
vpmullw ymm3{k7}, ymm4, ymm5
vpmullw zmm1, zmm2, zmmword ptr [rax+64]
vpmullw ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmullw xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmulld zmm0, zmm1, zmm2
vpmulld zmm31, zmm30, zmm29
vpmulld ymm8, ymm9, ymm16
vpmulld xmm16, xmm17, xmm18
vpmulld zmm3{k1}{z}, zmm20, zmm25
vpmulld ymm3{k7}, ymm4, ymm5
vpmulld zmm1, zmm2, zmmword ptr [rax+64]
vpmulld ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmulld xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmulld zmm1, zmm2, dword ptr [rax+4]{1to16}
vpmulld ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpmulld xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpmullq zmm0, zmm1, zmm2
vpmullq zmm31, zmm30, zmm29
vpmullq ymm8, ymm9, ymm16
vpmullq xmm16, xmm17, xmm18
vpmullq zmm3{k1}{z}, zmm20, zmm25
vpmullq ymm3{k7}, ymm4, ymm5
vpmullq zmm1, zmm2, zmmword ptr [rax+64]
vpmullq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmullq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmullq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpmullq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpmullq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpandd zmm0, zmm1, zmm2
vpandd zmm31, zmm30, zmm29
vpandd ymm8, ymm9, ymm16
vpandd xmm16, xmm17, xmm18
vpandd zmm3{k1}{z}, zmm20, zmm25
vpandd ymm3{k7}, ymm4, ymm5
vpandd zmm1, zmm2, zmmword ptr [rax+64]
vpandd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpandd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpandd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpandd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpandd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpandq zmm0, zmm1, zmm2
vpandq zmm31, zmm30, zmm29
vpandq ymm8, ymm9, ymm16
vpandq xmm16, xmm17, xmm18
vpandq zmm3{k1}{z}, zmm20, zmm25
vpandq ymm3{k7}, ymm4, ymm5
vpandq zmm1, zmm2, zmmword ptr [rax+64]
vpandq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpandq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpandq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpandq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpandq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpandnd zmm0, zmm1, zmm2
vpandnd zmm31, zmm30, zmm29
vpandnd ymm8, ymm9, ymm16
vpandnd xmm16, xmm17, xmm18
vpandnd zmm3{k1}{z}, zmm20, zmm25
vpandnd ymm3{k7}, ymm4, ymm5
vpandnd zmm1, zmm2, zmmword ptr [rax+64]
vpandnd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpandnd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpandnd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpandnd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpandnd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpandnq zmm0, zmm1, zmm2
vpandnq zmm31, zmm30, zmm29
vpandnq ymm8, ymm9, ymm16
vpandnq xmm16, xmm17, xmm18
vpandnq zmm3{k1}{z}, zmm20, zmm25
vpandnq ymm3{k7}, ymm4, ymm5
vpandnq zmm1, zmm2, zmmword ptr [rax+64]
vpandnq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpandnq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpandnq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpandnq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpandnq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpord zmm0, zmm1, zmm2
vpord zmm31, zmm30, zmm29
vpord ymm8, ymm9, ymm16
vpord xmm16, xmm17, xmm18
vpord zmm3{k1}{z}, zmm20, zmm25
vpord ymm3{k7}, ymm4, ymm5
vpord zmm1, zmm2, zmmword ptr [rax+64]
vpord ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpord xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpord zmm1, zmm2, dword ptr [rax+4]{1to16}
vpord ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpord xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vporq zmm0, zmm1, zmm2
vporq zmm31, zmm30, zmm29
vporq ymm8, ymm9, ymm16
vporq xmm16, xmm17, xmm18
vporq zmm3{k1}{z}, zmm20, zmm25
vporq ymm3{k7}, ymm4, ymm5
vporq zmm1, zmm2, zmmword ptr [rax+64]
vporq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vporq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vporq zmm1, zmm2, qword ptr [rax+8]{1to8}
vporq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vporq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpxord zmm0, zmm1, zmm2
vpxord zmm31, zmm30, zmm29
vpxord ymm8, ymm9, ymm16
vpxord xmm16, xmm17, xmm18
vpxord zmm3{k1}{z}, zmm20, zmm25
vpxord ymm3{k7}, ymm4, ymm5
vpxord zmm1, zmm2, zmmword ptr [rax+64]
vpxord ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpxord xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpxord zmm1, zmm2, dword ptr [rax+4]{1to16}
vpxord ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpxord xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpxorq zmm0, zmm1, zmm2
vpxorq zmm31, zmm30, zmm29
vpxorq ymm8, ymm9, ymm16
vpxorq xmm16, xmm17, xmm18
vpxorq zmm3{k1}{z}, zmm20, zmm25
vpxorq ymm3{k7}, ymm4, ymm5
vpxorq zmm1, zmm2, zmmword ptr [rax+64]
vpxorq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpxorq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpxorq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpxorq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpxorq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpminsd zmm0, zmm1, zmm2
vpminsd zmm31, zmm30, zmm29
vpminsd ymm8, ymm9, ymm16
vpminsd xmm16, xmm17, xmm18
vpminsd zmm3{k1}{z}, zmm20, zmm25
vpminsd ymm3{k7}, ymm4, ymm5
vpminsd zmm1, zmm2, zmmword ptr [rax+64]
vpminsd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpminsd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpminsd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpminsd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpminsd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpminsq zmm0, zmm1, zmm2
vpminsq zmm31, zmm30, zmm29
vpminsq ymm8, ymm9, ymm16
vpminsq xmm16, xmm17, xmm18
vpminsq zmm3{k1}{z}, zmm20, zmm25
vpminsq ymm3{k7}, ymm4, ymm5
vpminsq zmm1, zmm2, zmmword ptr [rax+64]
vpminsq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpminsq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpminsq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpminsq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpminsq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpminub zmm0, zmm1, zmm2
vpminub zmm31, zmm30, zmm29
vpminub ymm8, ymm9, ymm16
vpminub xmm16, xmm17, xmm18
vpminub zmm3{k1}{z}, zmm20, zmm25
vpminub ymm3{k7}, ymm4, ymm5
vpminub zmm1, zmm2, zmmword ptr [rax+64]
vpminub ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpminub xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpminuw zmm0, zmm1, zmm2
vpminuw zmm31, zmm30, zmm29
vpminuw ymm8, ymm9, ymm16
vpminuw xmm16, xmm17, xmm18
vpminuw zmm3{k1}{z}, zmm20, zmm25
vpminuw ymm3{k7}, ymm4, ymm5
vpminuw zmm1, zmm2, zmmword ptr [rax+64]
vpminuw ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpminuw xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpminud zmm0, zmm1, zmm2
vpminud zmm31, zmm30, zmm29
vpminud ymm8, ymm9, ymm16
vpminud xmm16, xmm17, xmm18
vpminud zmm3{k1}{z}, zmm20, zmm25
vpminud ymm3{k7}, ymm4, ymm5
vpminud zmm1, zmm2, zmmword ptr [rax+64]
vpminud ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpminud xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpminud zmm1, zmm2, dword ptr [rax+4]{1to16}
vpminud ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpminud xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpminuq zmm0, zmm1, zmm2
vpminuq zmm31, zmm30, zmm29
vpminuq ymm8, ymm9, ymm16
vpminuq xmm16, xmm17, xmm18
vpminuq zmm3{k1}{z}, zmm20, zmm25
vpminuq ymm3{k7}, ymm4, ymm5
vpminuq zmm1, zmm2, zmmword ptr [rax+64]
vpminuq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpminuq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpminuq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpminuq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpminuq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpmaxsd zmm0, zmm1, zmm2
vpmaxsd zmm31, zmm30, zmm29
vpmaxsd ymm8, ymm9, ymm16
vpmaxsd xmm16, xmm17, xmm18
vpmaxsd zmm3{k1}{z}, zmm20, zmm25
vpmaxsd ymm3{k7}, ymm4, ymm5
vpmaxsd zmm1, zmm2, zmmword ptr [rax+64]
vpmaxsd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmaxsd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmaxsd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpmaxsd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpmaxsd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpmaxsq zmm0, zmm1, zmm2
vpmaxsq zmm31, zmm30, zmm29
vpmaxsq ymm8, ymm9, ymm16
vpmaxsq xmm16, xmm17, xmm18
vpmaxsq zmm3{k1}{z}, zmm20, zmm25
vpmaxsq ymm3{k7}, ymm4, ymm5
vpmaxsq zmm1, zmm2, zmmword ptr [rax+64]
vpmaxsq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmaxsq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmaxsq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpmaxsq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpmaxsq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpmaxub zmm0, zmm1, zmm2
vpmaxub zmm31, zmm30, zmm29
vpmaxub ymm8, ymm9, ymm16
vpmaxub xmm16, xmm17, xmm18
vpmaxub zmm3{k1}{z}, zmm20, zmm25
vpmaxub ymm3{k7}, ymm4, ymm5
vpmaxub zmm1, zmm2, zmmword ptr [rax+64]
vpmaxub ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmaxub xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmaxuw zmm0, zmm1, zmm2
vpmaxuw zmm31, zmm30, zmm29
vpmaxuw ymm8, ymm9, ymm16
vpmaxuw xmm16, xmm17, xmm18
vpmaxuw zmm3{k1}{z}, zmm20, zmm25
vpmaxuw ymm3{k7}, ymm4, ymm5
vpmaxuw zmm1, zmm2, zmmword ptr [rax+64]
vpmaxuw ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmaxuw xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmaxud zmm0, zmm1, zmm2
vpmaxud zmm31, zmm30, zmm29
vpmaxud ymm8, ymm9, ymm16
vpmaxud xmm16, xmm17, xmm18
vpmaxud zmm3{k1}{z}, zmm20, zmm25
vpmaxud ymm3{k7}, ymm4, ymm5
vpmaxud zmm1, zmm2, zmmword ptr [rax+64]
vpmaxud ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmaxud xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmaxud zmm1, zmm2, dword ptr [rax+4]{1to16}
vpmaxud ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpmaxud xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpmaxuq zmm0, zmm1, zmm2
vpmaxuq zmm31, zmm30, zmm29
vpmaxuq ymm8, ymm9, ymm16
vpmaxuq xmm16, xmm17, xmm18
vpmaxuq zmm3{k1}{z}, zmm20, zmm25
vpmaxuq ymm3{k7}, ymm4, ymm5
vpmaxuq zmm1, zmm2, zmmword ptr [rax+64]
vpmaxuq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpmaxuq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmaxuq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpmaxuq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpmaxuq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpshufb zmm0, zmm1, zmm2
vpshufb zmm31, zmm30, zmm29
vpshufb ymm8, ymm9, ymm16
vpshufb xmm16, xmm17, xmm18
vpshufb zmm3{k1}{z}, zmm20, zmm25
vpshufb ymm3{k7}, ymm4, ymm5
vpshufb zmm1, zmm2, zmmword ptr [rax+64]
vpshufb ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpshufb xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpblendmb zmm0, zmm1, zmm2
vpblendmb zmm31, zmm30, zmm29
vpblendmb ymm8, ymm9, ymm16
vpblendmb xmm16, xmm17, xmm18
vpblendmb zmm3{k1}{z}, zmm20, zmm25
vpblendmb ymm3{k7}, ymm4, ymm5
vpblendmb zmm1, zmm2, zmmword ptr [rax+64]
vpblendmb ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpblendmb xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpblendmw zmm0, zmm1, zmm2
vpblendmw zmm31, zmm30, zmm29
vpblendmw ymm8, ymm9, ymm16
vpblendmw xmm16, xmm17, xmm18
vpblendmw zmm3{k1}{z}, zmm20, zmm25
vpblendmw ymm3{k7}, ymm4, ymm5
vpblendmw zmm1, zmm2, zmmword ptr [rax+64]
vpblendmw ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpblendmw xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpblendmd zmm0, zmm1, zmm2
vpblendmd zmm31, zmm30, zmm29
vpblendmd ymm8, ymm9, ymm16
vpblendmd xmm16, xmm17, xmm18
vpblendmd zmm3{k1}{z}, zmm20, zmm25
vpblendmd ymm3{k7}, ymm4, ymm5
vpblendmd zmm1, zmm2, zmmword ptr [rax+64]
vpblendmd ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpblendmd xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpblendmd zmm1, zmm2, dword ptr [rax+4]{1to16}
vpblendmd ymm9{k3}{z}, ymm28, dword ptr [rax+512]{1to8}
vpblendmd xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vpblendmq zmm0, zmm1, zmm2
vpblendmq zmm31, zmm30, zmm29
vpblendmq ymm8, ymm9, ymm16
vpblendmq xmm16, xmm17, xmm18
vpblendmq zmm3{k1}{z}, zmm20, zmm25
vpblendmq ymm3{k7}, ymm4, ymm5
vpblendmq zmm1, zmm2, zmmword ptr [rax+64]
vpblendmq ymm17{k2}, ymm18, ymmword ptr [r9+r15*4-128]
vpblendmq xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpblendmq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpblendmq ymm9{k3}{z}, ymm28, qword ptr [rax+1024]{1to4}
vpblendmq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
# addressing, full vector tuple
vpaddb xmm1, xmm2, xmmword ptr [rax]
vpmullq xmm17, xmm2, xmmword ptr [rax]
vpaddb xmm1, xmm2, xmmword ptr [rax+16]
vpmullq xmm17, xmm2, xmmword ptr [rax+16]
vpaddb xmm1, xmm2, xmmword ptr [rax+15]
vpmullq xmm17, xmm2, xmmword ptr [rax+15]
vpaddb xmm1, xmm2, xmmword ptr [rax-16]
vpmullq xmm17, xmm2, xmmword ptr [rax-16]
vpaddb xmm1, xmm2, xmmword ptr [rax+2032]
vpmullq xmm17, xmm2, xmmword ptr [rax+2032]
vpaddb xmm1, xmm2, xmmword ptr [rax+2048]
vpmullq xmm17, xmm2, xmmword ptr [rax+2048]
vpaddb xmm1, xmm2, xmmword ptr [rax-2048]
vpmullq xmm17, xmm2, xmmword ptr [rax-2048]
vpaddb xmm1, xmm2, xmmword ptr [rax-2064]
vpmullq xmm17, xmm2, xmmword ptr [rax-2064]
vpaddb xmm1, xmm2, xmmword ptr [rbp]
vpmullq xmm17, xmm2, xmmword ptr [rbp]
vpaddb xmm1, xmm2, xmmword ptr [r13]
vpmullq xmm17, xmm2, xmmword ptr [r13]
vpaddb xmm1, xmm2, xmmword ptr [rsp+32]
vpmullq xmm17, xmm2, xmmword ptr [rsp+32]
vpaddb xmm1, xmm2, xmmword ptr [r9+r15*4-128]
vpmullq xmm17, xmm2, xmmword ptr [r9+r15*4-128]
vpaddb xmm1, xmm2, xmmword ptr [rax+rcx*8+16]
vpmullq xmm17, xmm2, xmmword ptr [rax+rcx*8+16]
vpaddb xmm1, xmm2, xmmword ptr [r12+r13*2]
vpmullq xmm17, xmm2, xmmword ptr [r12+r13*2]
vpaddb xmm1, xmm2, xmmword ptr [rip+sym]
vpmullq xmm17, xmm2, xmmword ptr [rip+sym]
vpaddb xmm1, xmm2, xmmword ptr [rip+sym+16]
vpmullq xmm17, xmm2, xmmword ptr [rip+sym+16]
vpaddb xmm1, xmm2, xmmword ptr [0x1000]
vpmullq xmm17, xmm2, xmmword ptr [0x1000]
vpaddb xmm1, xmm2, xmmword ptr [rcx*4+16]
vpmullq xmm17, xmm2, xmmword ptr [rcx*4+16]
vpaddb ymm1, ymm2, ymmword ptr [rax]
vpmullq ymm17, ymm2, ymmword ptr [rax]
vpaddb ymm1, ymm2, ymmword ptr [rax+32]
vpmullq ymm17, ymm2, ymmword ptr [rax+32]
vpaddb ymm1, ymm2, ymmword ptr [rax+31]
vpmullq ymm17, ymm2, ymmword ptr [rax+31]
vpaddb ymm1, ymm2, ymmword ptr [rax-32]
vpmullq ymm17, ymm2, ymmword ptr [rax-32]
vpaddb ymm1, ymm2, ymmword ptr [rax+4064]
vpmullq ymm17, ymm2, ymmword ptr [rax+4064]
vpaddb ymm1, ymm2, ymmword ptr [rax+4096]
vpmullq ymm17, ymm2, ymmword ptr [rax+4096]
vpaddb ymm1, ymm2, ymmword ptr [rax-4096]
vpmullq ymm17, ymm2, ymmword ptr [rax-4096]
vpaddb ymm1, ymm2, ymmword ptr [rax-4128]
vpmullq ymm17, ymm2, ymmword ptr [rax-4128]
vpaddb ymm1, ymm2, ymmword ptr [rbp]
vpmullq ymm17, ymm2, ymmword ptr [rbp]
vpaddb ymm1, ymm2, ymmword ptr [r13]
vpmullq ymm17, ymm2, ymmword ptr [r13]
vpaddb ymm1, ymm2, ymmword ptr [rsp+64]
vpmullq ymm17, ymm2, ymmword ptr [rsp+64]
vpaddb ymm1, ymm2, ymmword ptr [r9+r15*4-256]
vpmullq ymm17, ymm2, ymmword ptr [r9+r15*4-256]
vpaddb ymm1, ymm2, ymmword ptr [rax+rcx*8+32]
vpmullq ymm17, ymm2, ymmword ptr [rax+rcx*8+32]
vpaddb ymm1, ymm2, ymmword ptr [r12+r13*2]
vpmullq ymm17, ymm2, ymmword ptr [r12+r13*2]
vpaddb ymm1, ymm2, ymmword ptr [rip+sym]
vpmullq ymm17, ymm2, ymmword ptr [rip+sym]
vpaddb ymm1, ymm2, ymmword ptr [rip+sym+32]
vpmullq ymm17, ymm2, ymmword ptr [rip+sym+32]
vpaddb ymm1, ymm2, ymmword ptr [0x1000]
vpmullq ymm17, ymm2, ymmword ptr [0x1000]
vpaddb ymm1, ymm2, ymmword ptr [rcx*4+32]
vpmullq ymm17, ymm2, ymmword ptr [rcx*4+32]
vpaddb zmm1, zmm2, zmmword ptr [rax]
vpmullq zmm17, zmm2, zmmword ptr [rax]
vpaddb zmm1, zmm2, zmmword ptr [rax+64]
vpmullq zmm17, zmm2, zmmword ptr [rax+64]
vpaddb zmm1, zmm2, zmmword ptr [rax+63]
vpmullq zmm17, zmm2, zmmword ptr [rax+63]
vpaddb zmm1, zmm2, zmmword ptr [rax-64]
vpmullq zmm17, zmm2, zmmword ptr [rax-64]
vpaddb zmm1, zmm2, zmmword ptr [rax+8128]
vpmullq zmm17, zmm2, zmmword ptr [rax+8128]
vpaddb zmm1, zmm2, zmmword ptr [rax+8192]
vpmullq zmm17, zmm2, zmmword ptr [rax+8192]
vpaddb zmm1, zmm2, zmmword ptr [rax-8192]
vpmullq zmm17, zmm2, zmmword ptr [rax-8192]
vpaddb zmm1, zmm2, zmmword ptr [rax-8256]
vpmullq zmm17, zmm2, zmmword ptr [rax-8256]
vpaddb zmm1, zmm2, zmmword ptr [rbp]
vpmullq zmm17, zmm2, zmmword ptr [rbp]
vpaddb zmm1, zmm2, zmmword ptr [r13]
vpmullq zmm17, zmm2, zmmword ptr [r13]
vpaddb zmm1, zmm2, zmmword ptr [rsp+128]
vpmullq zmm17, zmm2, zmmword ptr [rsp+128]
vpaddb zmm1, zmm2, zmmword ptr [r9+r15*4-512]
vpmullq zmm17, zmm2, zmmword ptr [r9+r15*4-512]
vpaddb zmm1, zmm2, zmmword ptr [rax+rcx*8+64]
vpmullq zmm17, zmm2, zmmword ptr [rax+rcx*8+64]
vpaddb zmm1, zmm2, zmmword ptr [r12+r13*2]
vpmullq zmm17, zmm2, zmmword ptr [r12+r13*2]
vpaddb zmm1, zmm2, zmmword ptr [rip+sym]
vpmullq zmm17, zmm2, zmmword ptr [rip+sym]
vpaddb zmm1, zmm2, zmmword ptr [rip+sym+64]
vpmullq zmm17, zmm2, zmmword ptr [rip+sym+64]
vpaddb zmm1, zmm2, zmmword ptr [0x1000]
vpmullq zmm17, zmm2, zmmword ptr [0x1000]
vpaddb zmm1, zmm2, zmmword ptr [rcx*4+64]
vpmullq zmm17, zmm2, zmmword ptr [rcx*4+64]
# addressing, broadcast tuple
vaddps xmm1, xmm2, dword ptr [rax]{1to4}
vaddps xmm1, xmm2, dword ptr [rax+4]{1to4}
vaddps xmm1, xmm2, dword ptr [rax+3]{1to4}
vaddps xmm1, xmm2, dword ptr [rax+508]{1to4}
vaddps xmm1, xmm2, dword ptr [rax+512]{1to4}
vaddps xmm1, xmm2, dword ptr [rax-512]{1to4}
vaddps xmm1, xmm2, dword ptr [rbp+-516]{1to4}
vaddps xmm1, xmm2, dword ptr [r10+rcx*2]{1to4}
vaddps xmm1, xmm2, dword ptr [rbp]{1to4}
vaddps xmm1, xmm2, dword ptr [rip+sym+4]{1to4}
vaddps ymm1, ymm2, dword ptr [rax]{1to8}
vaddps ymm1, ymm2, dword ptr [rax+4]{1to8}
vaddps ymm1, ymm2, dword ptr [rax+3]{1to8}
vaddps ymm1, ymm2, dword ptr [rax+508]{1to8}
vaddps ymm1, ymm2, dword ptr [rax+512]{1to8}
vaddps ymm1, ymm2, dword ptr [rax-512]{1to8}
vaddps ymm1, ymm2, dword ptr [rbp+-516]{1to8}
vaddps ymm1, ymm2, dword ptr [r10+rcx*2]{1to8}
vaddps ymm1, ymm2, dword ptr [rbp]{1to8}
vaddps ymm1, ymm2, dword ptr [rip+sym+4]{1to8}
vaddps zmm1, zmm2, dword ptr [rax]{1to16}
vaddps zmm1, zmm2, dword ptr [rax+4]{1to16}
vaddps zmm1, zmm2, dword ptr [rax+3]{1to16}
vaddps zmm1, zmm2, dword ptr [rax+508]{1to16}
vaddps zmm1, zmm2, dword ptr [rax+512]{1to16}
vaddps zmm1, zmm2, dword ptr [rax-512]{1to16}
vaddps zmm1, zmm2, dword ptr [rbp+-516]{1to16}
vaddps zmm1, zmm2, dword ptr [r10+rcx*2]{1to16}
vaddps zmm1, zmm2, dword ptr [rbp]{1to16}
vaddps zmm1, zmm2, dword ptr [rip+sym+4]{1to16}
vpaddq xmm1, xmm2, qword ptr [rax]{1to2}
vpaddq xmm1, xmm2, qword ptr [rax+8]{1to2}
vpaddq xmm1, xmm2, qword ptr [rax+7]{1to2}
vpaddq xmm1, xmm2, qword ptr [rax+1016]{1to2}
vpaddq xmm1, xmm2, qword ptr [rax+1024]{1to2}
vpaddq xmm1, xmm2, qword ptr [rax-1024]{1to2}
vpaddq xmm1, xmm2, qword ptr [rbp+-1032]{1to2}
vpaddq xmm1, xmm2, qword ptr [r10+rcx*2]{1to2}
vpaddq xmm1, xmm2, qword ptr [rbp]{1to2}
vpaddq xmm1, xmm2, qword ptr [rip+sym+8]{1to2}
vpaddq ymm1, ymm2, qword ptr [rax]{1to4}
vpaddq ymm1, ymm2, qword ptr [rax+8]{1to4}
vpaddq ymm1, ymm2, qword ptr [rax+7]{1to4}
vpaddq ymm1, ymm2, qword ptr [rax+1016]{1to4}
vpaddq ymm1, ymm2, qword ptr [rax+1024]{1to4}
vpaddq ymm1, ymm2, qword ptr [rax-1024]{1to4}
vpaddq ymm1, ymm2, qword ptr [rbp+-1032]{1to4}
vpaddq ymm1, ymm2, qword ptr [r10+rcx*2]{1to4}
vpaddq ymm1, ymm2, qword ptr [rbp]{1to4}
vpaddq ymm1, ymm2, qword ptr [rip+sym+8]{1to4}
vpaddq zmm1, zmm2, qword ptr [rax]{1to8}
vpaddq zmm1, zmm2, qword ptr [rax+8]{1to8}
vpaddq zmm1, zmm2, qword ptr [rax+7]{1to8}
vpaddq zmm1, zmm2, qword ptr [rax+1016]{1to8}
vpaddq zmm1, zmm2, qword ptr [rax+1024]{1to8}
vpaddq zmm1, zmm2, qword ptr [rax-1024]{1to8}
vpaddq zmm1, zmm2, qword ptr [rbp+-1032]{1to8}
vpaddq zmm1, zmm2, qword ptr [r10+rcx*2]{1to8}
vpaddq zmm1, zmm2, qword ptr [rbp]{1to8}
vpaddq zmm1, zmm2, qword ptr [rip+sym+8]{1to8}
# ternary logic
vpternlogd xmm0, xmm1, xmm2, 0x96
vpternlogd xmm31{k1}{z}, xmm30, xmm29, 0xe8
vpternlogd xmm8, xmm9, xmmword ptr [rax+16], 255
vpternlogd xmm8, xmm9, dword ptr [rax+4]{1to4}, 1
vpternlogd ymm0, ymm1, ymm2, 0x96
vpternlogd ymm31{k1}{z}, ymm30, ymm29, 0xe8
vpternlogd ymm8, ymm9, ymmword ptr [rax+32], 255
vpternlogd ymm8, ymm9, dword ptr [rax+4]{1to8}, 1
vpternlogd zmm0, zmm1, zmm2, 0x96
vpternlogd zmm31{k1}{z}, zmm30, zmm29, 0xe8
vpternlogd zmm8, zmm9, zmmword ptr [rax+64], 255
vpternlogd zmm8, zmm9, dword ptr [rax+4]{1to16}, 1
vpternlogq xmm0, xmm1, xmm2, 0x96
vpternlogq xmm31{k1}{z}, xmm30, xmm29, 0xe8
vpternlogq xmm8, xmm9, xmmword ptr [rax+16], 255
vpternlogq xmm8, xmm9, qword ptr [rax+8]{1to2}, 1
vpternlogq ymm0, ymm1, ymm2, 0x96
vpternlogq ymm31{k1}{z}, ymm30, ymm29, 0xe8
vpternlogq ymm8, ymm9, ymmword ptr [rax+32], 255
vpternlogq ymm8, ymm9, qword ptr [rax+8]{1to4}, 1
vpternlogq zmm0, zmm1, zmm2, 0x96
vpternlogq zmm31{k1}{z}, zmm30, zmm29, 0xe8
vpternlogq zmm8, zmm9, zmmword ptr [rax+64], 255
vpternlogq zmm8, zmm9, qword ptr [rax+8]{1to8}, 1
# compares into mask registers
vpcmpeqb k1, xmm0, xmm2
vpcmpeqb k7, xmm31, xmm30
vpcmpeqb k2, xmm16, xmm17
vpcmpeqb k1{k2}, xmm4, xmm5
vpcmpeqb k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpeqb k5, xmm8, xmmword ptr [rax+15]
vpcmpeqb k1, ymm0, ymm2
vpcmpeqb k7, ymm31, ymm30
vpcmpeqb k2, ymm16, ymm17
vpcmpeqb k1{k2}, ymm4, ymm5
vpcmpeqb k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpeqb k5, ymm8, ymmword ptr [rax+31]
vpcmpeqb k1, zmm0, zmm2
vpcmpeqb k7, zmm31, zmm30
vpcmpeqb k2, zmm16, zmm17
vpcmpeqb k1{k2}, zmm4, zmm5
vpcmpeqb k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpeqb k5, zmm8, zmmword ptr [rax+63]
vpcmpeqw k1, xmm0, xmm2
vpcmpeqw k7, xmm31, xmm30
vpcmpeqw k2, xmm16, xmm17
vpcmpeqw k1{k2}, xmm4, xmm5
vpcmpeqw k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpeqw k5, xmm8, xmmword ptr [rax+15]
vpcmpeqw k1, ymm0, ymm2
vpcmpeqw k7, ymm31, ymm30
vpcmpeqw k2, ymm16, ymm17
vpcmpeqw k1{k2}, ymm4, ymm5
vpcmpeqw k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpeqw k5, ymm8, ymmword ptr [rax+31]
vpcmpeqw k1, zmm0, zmm2
vpcmpeqw k7, zmm31, zmm30
vpcmpeqw k2, zmm16, zmm17
vpcmpeqw k1{k2}, zmm4, zmm5
vpcmpeqw k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpeqw k5, zmm8, zmmword ptr [rax+63]
vpcmpeqd k1, xmm0, xmm2
vpcmpeqd k7, xmm31, xmm30
vpcmpeqd k2, xmm16, xmm17
vpcmpeqd k1{k2}, xmm4, xmm5
vpcmpeqd k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpeqd k5, xmm8, xmmword ptr [rax+15]
vpcmpeqd k1, xmm2, dword ptr [rax+4]{1to4}
vpcmpeqd k1, ymm0, ymm2
vpcmpeqd k7, ymm31, ymm30
vpcmpeqd k2, ymm16, ymm17
vpcmpeqd k1{k2}, ymm4, ymm5
vpcmpeqd k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpeqd k5, ymm8, ymmword ptr [rax+31]
vpcmpeqd k1, ymm2, dword ptr [rax+4]{1to8}
vpcmpeqd k1, zmm0, zmm2
vpcmpeqd k7, zmm31, zmm30
vpcmpeqd k2, zmm16, zmm17
vpcmpeqd k1{k2}, zmm4, zmm5
vpcmpeqd k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpeqd k5, zmm8, zmmword ptr [rax+63]
vpcmpeqd k1, zmm2, dword ptr [rax+4]{1to16}
vpcmpeqq k1, xmm0, xmm2
vpcmpeqq k7, xmm31, xmm30
vpcmpeqq k2, xmm16, xmm17
vpcmpeqq k1{k2}, xmm4, xmm5
vpcmpeqq k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpeqq k5, xmm8, xmmword ptr [rax+15]
vpcmpeqq k1, xmm2, qword ptr [rax+8]{1to2}
vpcmpeqq k1, ymm0, ymm2
vpcmpeqq k7, ymm31, ymm30
vpcmpeqq k2, ymm16, ymm17
vpcmpeqq k1{k2}, ymm4, ymm5
vpcmpeqq k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpeqq k5, ymm8, ymmword ptr [rax+31]
vpcmpeqq k1, ymm2, qword ptr [rax+8]{1to4}
vpcmpeqq k1, zmm0, zmm2
vpcmpeqq k7, zmm31, zmm30
vpcmpeqq k2, zmm16, zmm17
vpcmpeqq k1{k2}, zmm4, zmm5
vpcmpeqq k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpeqq k5, zmm8, zmmword ptr [rax+63]
vpcmpeqq k1, zmm2, qword ptr [rax+8]{1to8}
vpcmpgtb k1, xmm0, xmm2
vpcmpgtb k7, xmm31, xmm30
vpcmpgtb k2, xmm16, xmm17
vpcmpgtb k1{k2}, xmm4, xmm5
vpcmpgtb k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpgtb k5, xmm8, xmmword ptr [rax+15]
vpcmpgtb k1, ymm0, ymm2
vpcmpgtb k7, ymm31, ymm30
vpcmpgtb k2, ymm16, ymm17
vpcmpgtb k1{k2}, ymm4, ymm5
vpcmpgtb k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpgtb k5, ymm8, ymmword ptr [rax+31]
vpcmpgtb k1, zmm0, zmm2
vpcmpgtb k7, zmm31, zmm30
vpcmpgtb k2, zmm16, zmm17
vpcmpgtb k1{k2}, zmm4, zmm5
vpcmpgtb k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpgtb k5, zmm8, zmmword ptr [rax+63]
vpcmpgtw k1, xmm0, xmm2
vpcmpgtw k7, xmm31, xmm30
vpcmpgtw k2, xmm16, xmm17
vpcmpgtw k1{k2}, xmm4, xmm5
vpcmpgtw k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpgtw k5, xmm8, xmmword ptr [rax+15]
vpcmpgtw k1, ymm0, ymm2
vpcmpgtw k7, ymm31, ymm30
vpcmpgtw k2, ymm16, ymm17
vpcmpgtw k1{k2}, ymm4, ymm5
vpcmpgtw k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpgtw k5, ymm8, ymmword ptr [rax+31]
vpcmpgtw k1, zmm0, zmm2
vpcmpgtw k7, zmm31, zmm30
vpcmpgtw k2, zmm16, zmm17
vpcmpgtw k1{k2}, zmm4, zmm5
vpcmpgtw k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpgtw k5, zmm8, zmmword ptr [rax+63]
vpcmpgtd k1, xmm0, xmm2
vpcmpgtd k7, xmm31, xmm30
vpcmpgtd k2, xmm16, xmm17
vpcmpgtd k1{k2}, xmm4, xmm5
vpcmpgtd k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpgtd k5, xmm8, xmmword ptr [rax+15]
vpcmpgtd k1, xmm2, dword ptr [rax+4]{1to4}
vpcmpgtd k1, ymm0, ymm2
vpcmpgtd k7, ymm31, ymm30
vpcmpgtd k2, ymm16, ymm17
vpcmpgtd k1{k2}, ymm4, ymm5
vpcmpgtd k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpgtd k5, ymm8, ymmword ptr [rax+31]
vpcmpgtd k1, ymm2, dword ptr [rax+4]{1to8}
vpcmpgtd k1, zmm0, zmm2
vpcmpgtd k7, zmm31, zmm30
vpcmpgtd k2, zmm16, zmm17
vpcmpgtd k1{k2}, zmm4, zmm5
vpcmpgtd k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpgtd k5, zmm8, zmmword ptr [rax+63]
vpcmpgtd k1, zmm2, dword ptr [rax+4]{1to16}
vpcmpgtq k1, xmm0, xmm2
vpcmpgtq k7, xmm31, xmm30
vpcmpgtq k2, xmm16, xmm17
vpcmpgtq k1{k2}, xmm4, xmm5
vpcmpgtq k5{k7}, xmm20, xmmword ptr [rax+16]
vpcmpgtq k5, xmm8, xmmword ptr [rax+15]
vpcmpgtq k1, xmm2, qword ptr [rax+8]{1to2}
vpcmpgtq k1, ymm0, ymm2
vpcmpgtq k7, ymm31, ymm30
vpcmpgtq k2, ymm16, ymm17
vpcmpgtq k1{k2}, ymm4, ymm5
vpcmpgtq k5{k7}, ymm20, ymmword ptr [rax+32]
vpcmpgtq k5, ymm8, ymmword ptr [rax+31]
vpcmpgtq k1, ymm2, qword ptr [rax+8]{1to4}
vpcmpgtq k1, zmm0, zmm2
vpcmpgtq k7, zmm31, zmm30
vpcmpgtq k2, zmm16, zmm17
vpcmpgtq k1{k2}, zmm4, zmm5
vpcmpgtq k5{k7}, zmm20, zmmword ptr [rax+64]
vpcmpgtq k5, zmm8, zmmword ptr [rax+63]
vpcmpgtq k1, zmm2, qword ptr [rax+8]{1to8}
vpcmpb k1, xmm0, xmm2, 3
vpcmpb k7, xmm31, xmm30, 3
vpcmpb k2, xmm16, xmm17, 3
vpcmpb k1{k2}, xmm4, xmm5, 3
vpcmpb k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpb k5, xmm8, xmmword ptr [rax+15], 3
vpcmpb k1, ymm0, ymm2, 3
vpcmpb k7, ymm31, ymm30, 3
vpcmpb k2, ymm16, ymm17, 3
vpcmpb k1{k2}, ymm4, ymm5, 3
vpcmpb k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpb k5, ymm8, ymmword ptr [rax+31], 3
vpcmpb k1, zmm0, zmm2, 3
vpcmpb k7, zmm31, zmm30, 3
vpcmpb k2, zmm16, zmm17, 3
vpcmpb k1{k2}, zmm4, zmm5, 3
vpcmpb k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpb k5, zmm8, zmmword ptr [rax+63], 3
vpcmpub k1, xmm0, xmm2, 3
vpcmpub k7, xmm31, xmm30, 3
vpcmpub k2, xmm16, xmm17, 3
vpcmpub k1{k2}, xmm4, xmm5, 3
vpcmpub k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpub k5, xmm8, xmmword ptr [rax+15], 3
vpcmpub k1, ymm0, ymm2, 3
vpcmpub k7, ymm31, ymm30, 3
vpcmpub k2, ymm16, ymm17, 3
vpcmpub k1{k2}, ymm4, ymm5, 3
vpcmpub k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpub k5, ymm8, ymmword ptr [rax+31], 3
vpcmpub k1, zmm0, zmm2, 3
vpcmpub k7, zmm31, zmm30, 3
vpcmpub k2, zmm16, zmm17, 3
vpcmpub k1{k2}, zmm4, zmm5, 3
vpcmpub k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpub k5, zmm8, zmmword ptr [rax+63], 3
vpcmpw k1, xmm0, xmm2, 3
vpcmpw k7, xmm31, xmm30, 3
vpcmpw k2, xmm16, xmm17, 3
vpcmpw k1{k2}, xmm4, xmm5, 3
vpcmpw k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpw k5, xmm8, xmmword ptr [rax+15], 3
vpcmpw k1, ymm0, ymm2, 3
vpcmpw k7, ymm31, ymm30, 3
vpcmpw k2, ymm16, ymm17, 3
vpcmpw k1{k2}, ymm4, ymm5, 3
vpcmpw k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpw k5, ymm8, ymmword ptr [rax+31], 3
vpcmpw k1, zmm0, zmm2, 3
vpcmpw k7, zmm31, zmm30, 3
vpcmpw k2, zmm16, zmm17, 3
vpcmpw k1{k2}, zmm4, zmm5, 3
vpcmpw k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpw k5, zmm8, zmmword ptr [rax+63], 3
vpcmpuw k1, xmm0, xmm2, 3
vpcmpuw k7, xmm31, xmm30, 3
vpcmpuw k2, xmm16, xmm17, 3
vpcmpuw k1{k2}, xmm4, xmm5, 3
vpcmpuw k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpuw k5, xmm8, xmmword ptr [rax+15], 3
vpcmpuw k1, ymm0, ymm2, 3
vpcmpuw k7, ymm31, ymm30, 3
vpcmpuw k2, ymm16, ymm17, 3
vpcmpuw k1{k2}, ymm4, ymm5, 3
vpcmpuw k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpuw k5, ymm8, ymmword ptr [rax+31], 3
vpcmpuw k1, zmm0, zmm2, 3
vpcmpuw k7, zmm31, zmm30, 3
vpcmpuw k2, zmm16, zmm17, 3
vpcmpuw k1{k2}, zmm4, zmm5, 3
vpcmpuw k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpuw k5, zmm8, zmmword ptr [rax+63], 3
vpcmpd k1, xmm0, xmm2, 3
vpcmpd k7, xmm31, xmm30, 3
vpcmpd k2, xmm16, xmm17, 3
vpcmpd k1{k2}, xmm4, xmm5, 3
vpcmpd k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpd k5, xmm8, xmmword ptr [rax+15], 3
vpcmpd k1, xmm2, dword ptr [rax+4]{1to4}, 3
vpcmpd k1, ymm0, ymm2, 3
vpcmpd k7, ymm31, ymm30, 3
vpcmpd k2, ymm16, ymm17, 3
vpcmpd k1{k2}, ymm4, ymm5, 3
vpcmpd k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpd k5, ymm8, ymmword ptr [rax+31], 3
vpcmpd k1, ymm2, dword ptr [rax+4]{1to8}, 3
vpcmpd k1, zmm0, zmm2, 3
vpcmpd k7, zmm31, zmm30, 3
vpcmpd k2, zmm16, zmm17, 3
vpcmpd k1{k2}, zmm4, zmm5, 3
vpcmpd k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpd k5, zmm8, zmmword ptr [rax+63], 3
vpcmpd k1, zmm2, dword ptr [rax+4]{1to16}, 3
vpcmpud k1, xmm0, xmm2, 3
vpcmpud k7, xmm31, xmm30, 3
vpcmpud k2, xmm16, xmm17, 3
vpcmpud k1{k2}, xmm4, xmm5, 3
vpcmpud k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpud k5, xmm8, xmmword ptr [rax+15], 3
vpcmpud k1, xmm2, dword ptr [rax+4]{1to4}, 3
vpcmpud k1, ymm0, ymm2, 3
vpcmpud k7, ymm31, ymm30, 3
vpcmpud k2, ymm16, ymm17, 3
vpcmpud k1{k2}, ymm4, ymm5, 3
vpcmpud k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpud k5, ymm8, ymmword ptr [rax+31], 3
vpcmpud k1, ymm2, dword ptr [rax+4]{1to8}, 3
vpcmpud k1, zmm0, zmm2, 3
vpcmpud k7, zmm31, zmm30, 3
vpcmpud k2, zmm16, zmm17, 3
vpcmpud k1{k2}, zmm4, zmm5, 3
vpcmpud k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpud k5, zmm8, zmmword ptr [rax+63], 3
vpcmpud k1, zmm2, dword ptr [rax+4]{1to16}, 3
vpcmpq k1, xmm0, xmm2, 3
vpcmpq k7, xmm31, xmm30, 3
vpcmpq k2, xmm16, xmm17, 3
vpcmpq k1{k2}, xmm4, xmm5, 3
vpcmpq k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpq k5, xmm8, xmmword ptr [rax+15], 3
vpcmpq k1, xmm2, qword ptr [rax+8]{1to2}, 3
vpcmpq k1, ymm0, ymm2, 3
vpcmpq k7, ymm31, ymm30, 3
vpcmpq k2, ymm16, ymm17, 3
vpcmpq k1{k2}, ymm4, ymm5, 3
vpcmpq k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpq k5, ymm8, ymmword ptr [rax+31], 3
vpcmpq k1, ymm2, qword ptr [rax+8]{1to4}, 3
vpcmpq k1, zmm0, zmm2, 3
vpcmpq k7, zmm31, zmm30, 3
vpcmpq k2, zmm16, zmm17, 3
vpcmpq k1{k2}, zmm4, zmm5, 3
vpcmpq k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpq k5, zmm8, zmmword ptr [rax+63], 3
vpcmpq k1, zmm2, qword ptr [rax+8]{1to8}, 3
vpcmpuq k1, xmm0, xmm2, 3
vpcmpuq k7, xmm31, xmm30, 3
vpcmpuq k2, xmm16, xmm17, 3
vpcmpuq k1{k2}, xmm4, xmm5, 3
vpcmpuq k5{k7}, xmm20, xmmword ptr [rax+16], 3
vpcmpuq k5, xmm8, xmmword ptr [rax+15], 3
vpcmpuq k1, xmm2, qword ptr [rax+8]{1to2}, 3
vpcmpuq k1, ymm0, ymm2, 3
vpcmpuq k7, ymm31, ymm30, 3
vpcmpuq k2, ymm16, ymm17, 3
vpcmpuq k1{k2}, ymm4, ymm5, 3
vpcmpuq k5{k7}, ymm20, ymmword ptr [rax+32], 3
vpcmpuq k5, ymm8, ymmword ptr [rax+31], 3
vpcmpuq k1, ymm2, qword ptr [rax+8]{1to4}, 3
vpcmpuq k1, zmm0, zmm2, 3
vpcmpuq k7, zmm31, zmm30, 3
vpcmpuq k2, zmm16, zmm17, 3
vpcmpuq k1{k2}, zmm4, zmm5, 3
vpcmpuq k5{k7}, zmm20, zmmword ptr [rax+64], 3
vpcmpuq k5, zmm8, zmmword ptr [rax+63], 3
vpcmpuq k1, zmm2, qword ptr [rax+8]{1to8}, 3
vcmpps k1, xmm0, xmm2, 3
vcmpps k7, xmm31, xmm30, 3
vcmpps k2, xmm16, xmm17, 3
vcmpps k1{k2}, xmm4, xmm5, 3
vcmpps k5{k7}, xmm20, xmmword ptr [rax+16], 3
vcmpps k5, xmm8, xmmword ptr [rax+15], 3
vcmpps k1, xmm2, dword ptr [rax+4]{1to4}, 3
vcmpps k1, ymm0, ymm2, 3
vcmpps k7, ymm31, ymm30, 3
vcmpps k2, ymm16, ymm17, 3
vcmpps k1{k2}, ymm4, ymm5, 3
vcmpps k5{k7}, ymm20, ymmword ptr [rax+32], 3
vcmpps k5, ymm8, ymmword ptr [rax+31], 3
vcmpps k1, ymm2, dword ptr [rax+4]{1to8}, 3
vcmpps k1, zmm0, zmm2, 3
vcmpps k7, zmm31, zmm30, 3
vcmpps k2, zmm16, zmm17, 3
vcmpps k1{k2}, zmm4, zmm5, 3
vcmpps k5{k7}, zmm20, zmmword ptr [rax+64], 3
vcmpps k5, zmm8, zmmword ptr [rax+63], 3
vcmpps k1, zmm2, dword ptr [rax+4]{1to16}, 3
vcmppd k1, xmm0, xmm2, 3
vcmppd k7, xmm31, xmm30, 3
vcmppd k2, xmm16, xmm17, 3
vcmppd k1{k2}, xmm4, xmm5, 3
vcmppd k5{k7}, xmm20, xmmword ptr [rax+16], 3
vcmppd k5, xmm8, xmmword ptr [rax+15], 3
vcmppd k1, xmm2, qword ptr [rax+8]{1to2}, 3
vcmppd k1, ymm0, ymm2, 3
vcmppd k7, ymm31, ymm30, 3
vcmppd k2, ymm16, ymm17, 3
vcmppd k1{k2}, ymm4, ymm5, 3
vcmppd k5{k7}, ymm20, ymmword ptr [rax+32], 3
vcmppd k5, ymm8, ymmword ptr [rax+31], 3
vcmppd k1, ymm2, qword ptr [rax+8]{1to4}, 3
vcmppd k1, zmm0, zmm2, 3
vcmppd k7, zmm31, zmm30, 3
vcmppd k2, zmm16, zmm17, 3
vcmppd k1{k2}, zmm4, zmm5, 3
vcmppd k5{k7}, zmm20, zmmword ptr [rax+64], 3
vcmppd k5, zmm8, zmmword ptr [rax+63], 3
vcmppd k1, zmm2, qword ptr [rax+8]{1to8}, 3
# compare predicates
vpcmpb k1, zmm2, zmm3, 0
vpcmpb k1, zmm2, zmm3, 1
vpcmpb k1, zmm2, zmm3, 2
vpcmpb k1, zmm2, zmm3, 4
vpcmpb k1, zmm2, zmm3, 5
vpcmpb k1, zmm2, zmm3, 6
vpcmpb k1, zmm2, zmm3, 7
vpcmpb k1, zmm2, zmm3, 31
vpcmpub k1, zmm2, zmm3, 0
vpcmpub k1, zmm2, zmm3, 1
vpcmpub k1, zmm2, zmm3, 2
vpcmpub k1, zmm2, zmm3, 4
vpcmpub k1, zmm2, zmm3, 5
vpcmpub k1, zmm2, zmm3, 6
vpcmpub k1, zmm2, zmm3, 7
vpcmpub k1, zmm2, zmm3, 31
vpcmpd k1, zmm2, zmm3, 0
vpcmpd k1, zmm2, zmm3, 1
vpcmpd k1, zmm2, zmm3, 2
vpcmpd k1, zmm2, zmm3, 4
vpcmpd k1, zmm2, zmm3, 5
vpcmpd k1, zmm2, zmm3, 6
vpcmpd k1, zmm2, zmm3, 7
vpcmpd k1, zmm2, zmm3, 31
vpcmpuq k1, zmm2, zmm3, 0
vpcmpuq k1, zmm2, zmm3, 1
vpcmpuq k1, zmm2, zmm3, 2
vpcmpuq k1, zmm2, zmm3, 4
vpcmpuq k1, zmm2, zmm3, 5
vpcmpuq k1, zmm2, zmm3, 6
vpcmpuq k1, zmm2, zmm3, 7
vpcmpuq k1, zmm2, zmm3, 31
vcmpps k1, zmm2, zmm3, 0
vcmpps k1, zmm2, zmm3, 1
vcmpps k1, zmm2, zmm3, 2
vcmpps k1, zmm2, zmm3, 4
vcmpps k1, zmm2, zmm3, 5
vcmpps k1, zmm2, zmm3, 6
vcmpps k1, zmm2, zmm3, 7
vcmpps k1, zmm2, zmm3, 31
vcmppd k1, zmm2, zmm3, 0
vcmppd k1, zmm2, zmm3, 1
vcmppd k1, zmm2, zmm3, 2
vcmppd k1, zmm2, zmm3, 4
vcmppd k1, zmm2, zmm3, 5
vcmppd k1, zmm2, zmm3, 6
vcmppd k1, zmm2, zmm3, 7
vcmppd k1, zmm2, zmm3, 31
# shifts by an xmm count
vpsllw xmm0, xmm1, xmm2
vpsllw xmm16, xmm17, xmm18
vpsllw xmm8{k1}{z}, xmm9, xmm31
vpsllw xmm8, xmm9, xmmword ptr [rax+16]
vpsllw xmm8, xmm9, xmmword ptr [rax+15]
vpsllw xmm8, xmm9, xmmword ptr [rax+2032]
vpsllw xmm8, xmm9, xmmword ptr [rax+2048]
vpsllw xmm8, xmm9, xmmword ptr [r13-2048]
vpsllw ymm0, ymm1, xmm2
vpsllw ymm16, ymm17, xmm18
vpsllw ymm8{k1}{z}, ymm9, xmm31
vpsllw ymm8, ymm9, xmmword ptr [rax+16]
vpsllw ymm8, ymm9, xmmword ptr [rax+15]
vpsllw ymm8, ymm9, xmmword ptr [rax+2032]
vpsllw ymm8, ymm9, xmmword ptr [rax+2048]
vpsllw ymm8, ymm9, xmmword ptr [r13-2048]
vpsllw zmm0, zmm1, xmm2
vpsllw zmm16, zmm17, xmm18
vpsllw zmm8{k1}{z}, zmm9, xmm31
vpsllw zmm8, zmm9, xmmword ptr [rax+16]
vpsllw zmm8, zmm9, xmmword ptr [rax+15]
vpsllw zmm8, zmm9, xmmword ptr [rax+2032]
vpsllw zmm8, zmm9, xmmword ptr [rax+2048]
vpsllw zmm8, zmm9, xmmword ptr [r13-2048]
vpslld xmm0, xmm1, xmm2
vpslld xmm16, xmm17, xmm18
vpslld xmm8{k1}{z}, xmm9, xmm31
vpslld xmm8, xmm9, xmmword ptr [rax+16]
vpslld xmm8, xmm9, xmmword ptr [rax+15]
vpslld xmm8, xmm9, xmmword ptr [rax+2032]
vpslld xmm8, xmm9, xmmword ptr [rax+2048]
vpslld xmm8, xmm9, xmmword ptr [r13-2048]
vpslld ymm0, ymm1, xmm2
vpslld ymm16, ymm17, xmm18
vpslld ymm8{k1}{z}, ymm9, xmm31
vpslld ymm8, ymm9, xmmword ptr [rax+16]
vpslld ymm8, ymm9, xmmword ptr [rax+15]
vpslld ymm8, ymm9, xmmword ptr [rax+2032]
vpslld ymm8, ymm9, xmmword ptr [rax+2048]
vpslld ymm8, ymm9, xmmword ptr [r13-2048]
vpslld zmm0, zmm1, xmm2
vpslld zmm16, zmm17, xmm18
vpslld zmm8{k1}{z}, zmm9, xmm31
vpslld zmm8, zmm9, xmmword ptr [rax+16]
vpslld zmm8, zmm9, xmmword ptr [rax+15]
vpslld zmm8, zmm9, xmmword ptr [rax+2032]
vpslld zmm8, zmm9, xmmword ptr [rax+2048]
vpslld zmm8, zmm9, xmmword ptr [r13-2048]
vpsllq xmm0, xmm1, xmm2
vpsllq xmm16, xmm17, xmm18
vpsllq xmm8{k1}{z}, xmm9, xmm31
vpsllq xmm8, xmm9, xmmword ptr [rax+16]
vpsllq xmm8, xmm9, xmmword ptr [rax+15]
vpsllq xmm8, xmm9, xmmword ptr [rax+2032]
vpsllq xmm8, xmm9, xmmword ptr [rax+2048]
vpsllq xmm8, xmm9, xmmword ptr [r13-2048]
vpsllq ymm0, ymm1, xmm2
vpsllq ymm16, ymm17, xmm18
vpsllq ymm8{k1}{z}, ymm9, xmm31
vpsllq ymm8, ymm9, xmmword ptr [rax+16]
vpsllq ymm8, ymm9, xmmword ptr [rax+15]
vpsllq ymm8, ymm9, xmmword ptr [rax+2032]
vpsllq ymm8, ymm9, xmmword ptr [rax+2048]
vpsllq ymm8, ymm9, xmmword ptr [r13-2048]
vpsllq zmm0, zmm1, xmm2
vpsllq zmm16, zmm17, xmm18
vpsllq zmm8{k1}{z}, zmm9, xmm31
vpsllq zmm8, zmm9, xmmword ptr [rax+16]
vpsllq zmm8, zmm9, xmmword ptr [rax+15]
vpsllq zmm8, zmm9, xmmword ptr [rax+2032]
vpsllq zmm8, zmm9, xmmword ptr [rax+2048]
vpsllq zmm8, zmm9, xmmword ptr [r13-2048]
vpsrlw xmm0, xmm1, xmm2
vpsrlw xmm16, xmm17, xmm18
vpsrlw xmm8{k1}{z}, xmm9, xmm31
vpsrlw xmm8, xmm9, xmmword ptr [rax+16]
vpsrlw xmm8, xmm9, xmmword ptr [rax+15]
vpsrlw xmm8, xmm9, xmmword ptr [rax+2032]
vpsrlw xmm8, xmm9, xmmword ptr [rax+2048]
vpsrlw xmm8, xmm9, xmmword ptr [r13-2048]
vpsrlw ymm0, ymm1, xmm2
vpsrlw ymm16, ymm17, xmm18
vpsrlw ymm8{k1}{z}, ymm9, xmm31
vpsrlw ymm8, ymm9, xmmword ptr [rax+16]
vpsrlw ymm8, ymm9, xmmword ptr [rax+15]
vpsrlw ymm8, ymm9, xmmword ptr [rax+2032]
vpsrlw ymm8, ymm9, xmmword ptr [rax+2048]
vpsrlw ymm8, ymm9, xmmword ptr [r13-2048]
vpsrlw zmm0, zmm1, xmm2
vpsrlw zmm16, zmm17, xmm18
vpsrlw zmm8{k1}{z}, zmm9, xmm31
vpsrlw zmm8, zmm9, xmmword ptr [rax+16]
vpsrlw zmm8, zmm9, xmmword ptr [rax+15]
vpsrlw zmm8, zmm9, xmmword ptr [rax+2032]
vpsrlw zmm8, zmm9, xmmword ptr [rax+2048]
vpsrlw zmm8, zmm9, xmmword ptr [r13-2048]
vpsrld xmm0, xmm1, xmm2
vpsrld xmm16, xmm17, xmm18
vpsrld xmm8{k1}{z}, xmm9, xmm31
vpsrld xmm8, xmm9, xmmword ptr [rax+16]
vpsrld xmm8, xmm9, xmmword ptr [rax+15]
vpsrld xmm8, xmm9, xmmword ptr [rax+2032]
vpsrld xmm8, xmm9, xmmword ptr [rax+2048]
vpsrld xmm8, xmm9, xmmword ptr [r13-2048]
vpsrld ymm0, ymm1, xmm2
vpsrld ymm16, ymm17, xmm18
vpsrld ymm8{k1}{z}, ymm9, xmm31
vpsrld ymm8, ymm9, xmmword ptr [rax+16]
vpsrld ymm8, ymm9, xmmword ptr [rax+15]
vpsrld ymm8, ymm9, xmmword ptr [rax+2032]
vpsrld ymm8, ymm9, xmmword ptr [rax+2048]
vpsrld ymm8, ymm9, xmmword ptr [r13-2048]
vpsrld zmm0, zmm1, xmm2
vpsrld zmm16, zmm17, xmm18
vpsrld zmm8{k1}{z}, zmm9, xmm31
vpsrld zmm8, zmm9, xmmword ptr [rax+16]
vpsrld zmm8, zmm9, xmmword ptr [rax+15]
vpsrld zmm8, zmm9, xmmword ptr [rax+2032]
vpsrld zmm8, zmm9, xmmword ptr [rax+2048]
vpsrld zmm8, zmm9, xmmword ptr [r13-2048]
vpsrlq xmm0, xmm1, xmm2
vpsrlq xmm16, xmm17, xmm18
vpsrlq xmm8{k1}{z}, xmm9, xmm31
vpsrlq xmm8, xmm9, xmmword ptr [rax+16]
vpsrlq xmm8, xmm9, xmmword ptr [rax+15]
vpsrlq xmm8, xmm9, xmmword ptr [rax+2032]
vpsrlq xmm8, xmm9, xmmword ptr [rax+2048]
vpsrlq xmm8, xmm9, xmmword ptr [r13-2048]
vpsrlq ymm0, ymm1, xmm2
vpsrlq ymm16, ymm17, xmm18
vpsrlq ymm8{k1}{z}, ymm9, xmm31
vpsrlq ymm8, ymm9, xmmword ptr [rax+16]
vpsrlq ymm8, ymm9, xmmword ptr [rax+15]
vpsrlq ymm8, ymm9, xmmword ptr [rax+2032]
vpsrlq ymm8, ymm9, xmmword ptr [rax+2048]
vpsrlq ymm8, ymm9, xmmword ptr [r13-2048]
vpsrlq zmm0, zmm1, xmm2
vpsrlq zmm16, zmm17, xmm18
vpsrlq zmm8{k1}{z}, zmm9, xmm31
vpsrlq zmm8, zmm9, xmmword ptr [rax+16]
vpsrlq zmm8, zmm9, xmmword ptr [rax+15]
vpsrlq zmm8, zmm9, xmmword ptr [rax+2032]
vpsrlq zmm8, zmm9, xmmword ptr [rax+2048]
vpsrlq zmm8, zmm9, xmmword ptr [r13-2048]
vpsrad xmm0, xmm1, xmm2
vpsrad xmm16, xmm17, xmm18
vpsrad xmm8{k1}{z}, xmm9, xmm31
vpsrad xmm8, xmm9, xmmword ptr [rax+16]
vpsrad xmm8, xmm9, xmmword ptr [rax+15]
vpsrad xmm8, xmm9, xmmword ptr [rax+2032]
vpsrad xmm8, xmm9, xmmword ptr [rax+2048]
vpsrad xmm8, xmm9, xmmword ptr [r13-2048]
vpsrad ymm0, ymm1, xmm2
vpsrad ymm16, ymm17, xmm18
vpsrad ymm8{k1}{z}, ymm9, xmm31
vpsrad ymm8, ymm9, xmmword ptr [rax+16]
vpsrad ymm8, ymm9, xmmword ptr [rax+15]
vpsrad ymm8, ymm9, xmmword ptr [rax+2032]
vpsrad ymm8, ymm9, xmmword ptr [rax+2048]
vpsrad ymm8, ymm9, xmmword ptr [r13-2048]
vpsrad zmm0, zmm1, xmm2
vpsrad zmm16, zmm17, xmm18
vpsrad zmm8{k1}{z}, zmm9, xmm31
vpsrad zmm8, zmm9, xmmword ptr [rax+16]
vpsrad zmm8, zmm9, xmmword ptr [rax+15]
vpsrad zmm8, zmm9, xmmword ptr [rax+2032]
vpsrad zmm8, zmm9, xmmword ptr [rax+2048]
vpsrad zmm8, zmm9, xmmword ptr [r13-2048]
vpsraq xmm0, xmm1, xmm2
vpsraq xmm16, xmm17, xmm18
vpsraq xmm8{k1}{z}, xmm9, xmm31
vpsraq xmm8, xmm9, xmmword ptr [rax+16]
vpsraq xmm8, xmm9, xmmword ptr [rax+15]
vpsraq xmm8, xmm9, xmmword ptr [rax+2032]
vpsraq xmm8, xmm9, xmmword ptr [rax+2048]
vpsraq xmm8, xmm9, xmmword ptr [r13-2048]
vpsraq ymm0, ymm1, xmm2
vpsraq ymm16, ymm17, xmm18
vpsraq ymm8{k1}{z}, ymm9, xmm31
vpsraq ymm8, ymm9, xmmword ptr [rax+16]
vpsraq ymm8, ymm9, xmmword ptr [rax+15]
vpsraq ymm8, ymm9, xmmword ptr [rax+2032]
vpsraq ymm8, ymm9, xmmword ptr [rax+2048]
vpsraq ymm8, ymm9, xmmword ptr [r13-2048]
vpsraq zmm0, zmm1, xmm2
vpsraq zmm16, zmm17, xmm18
vpsraq zmm8{k1}{z}, zmm9, xmm31
vpsraq zmm8, zmm9, xmmword ptr [rax+16]
vpsraq zmm8, zmm9, xmmword ptr [rax+15]
vpsraq zmm8, zmm9, xmmword ptr [rax+2032]
vpsraq zmm8, zmm9, xmmword ptr [rax+2048]
vpsraq zmm8, zmm9, xmmword ptr [r13-2048]
# moves
vmovdqa32 xmm0, xmm1
vmovdqa32 xmm16, xmm31
vmovdqa32 xmm8{k1}{z}, xmm9
vmovdqa32 xmm8{k1}, xmm25
vmovdqa32 xmm1, xmmword ptr [rax]
vmovdqa32 xmmword ptr [rax], xmm2
vmovdqa32 xmm1, xmmword ptr [rax+16]
vmovdqa32 xmmword ptr [rax+16], xmm2
vmovdqa32 xmm1, xmmword ptr [rax+15]
vmovdqa32 xmmword ptr [rax+15], xmm2
vmovdqa32 xmm1, xmmword ptr [rax-16]
vmovdqa32 xmmword ptr [rax-16], xmm2
vmovdqa32 xmm1, xmmword ptr [rax+2032]
vmovdqa32 xmmword ptr [rax+2032], xmm2
vmovdqa32 xmm1, xmmword ptr [rax+2048]
vmovdqa32 xmmword ptr [rax+2048], xmm2
vmovdqa32 xmm1, xmmword ptr [rbp]
vmovdqa32 xmmword ptr [rbp], xmm2
vmovdqa32 xmm1, xmmword ptr [r13]
vmovdqa32 xmmword ptr [r13], xmm2
vmovdqa32 xmm1, xmmword ptr [rsp+32]
vmovdqa32 xmmword ptr [rsp+32], xmm2
vmovdqa32 xmm1, xmmword ptr [r9+r15*4-128]
vmovdqa32 xmmword ptr [r9+r15*4-128], xmm2
vmovdqa32 xmm1, xmmword ptr [rip+sym]
vmovdqa32 xmmword ptr [rip+sym], xmm2
vmovdqa32 xmm20{k3}{z}, xmmword ptr [rax+16]
vmovdqa32 xmmword ptr [r9+rdx*8-32]{k6}, xmm31
vmovdqa32 ymm0, ymm1
vmovdqa32 ymm16, ymm31
vmovdqa32 ymm8{k1}{z}, ymm9
vmovdqa32 ymm8{k1}, ymm25
vmovdqa32 ymm1, ymmword ptr [rax]
vmovdqa32 ymmword ptr [rax], ymm2
vmovdqa32 ymm1, ymmword ptr [rax+32]
vmovdqa32 ymmword ptr [rax+32], ymm2
vmovdqa32 ymm1, ymmword ptr [rax+31]
vmovdqa32 ymmword ptr [rax+31], ymm2
vmovdqa32 ymm1, ymmword ptr [rax-32]
vmovdqa32 ymmword ptr [rax-32], ymm2
vmovdqa32 ymm1, ymmword ptr [rax+4064]
vmovdqa32 ymmword ptr [rax+4064], ymm2
vmovdqa32 ymm1, ymmword ptr [rax+4096]
vmovdqa32 ymmword ptr [rax+4096], ymm2
vmovdqa32 ymm1, ymmword ptr [rbp]
vmovdqa32 ymmword ptr [rbp], ymm2
vmovdqa32 ymm1, ymmword ptr [r13]
vmovdqa32 ymmword ptr [r13], ymm2
vmovdqa32 ymm1, ymmword ptr [rsp+64]
vmovdqa32 ymmword ptr [rsp+64], ymm2
vmovdqa32 ymm1, ymmword ptr [r9+r15*4-256]
vmovdqa32 ymmword ptr [r9+r15*4-256], ymm2
vmovdqa32 ymm1, ymmword ptr [rip+sym]
vmovdqa32 ymmword ptr [rip+sym], ymm2
vmovdqa32 ymm20{k3}{z}, ymmword ptr [rax+32]
vmovdqa32 ymmword ptr [r9+rdx*8-64]{k6}, ymm31
vmovdqa32 zmm0, zmm1
vmovdqa32 zmm16, zmm31
vmovdqa32 zmm8{k1}{z}, zmm9
vmovdqa32 zmm8{k1}, zmm25
vmovdqa32 zmm1, zmmword ptr [rax]
vmovdqa32 zmmword ptr [rax], zmm2
vmovdqa32 zmm1, zmmword ptr [rax+64]
vmovdqa32 zmmword ptr [rax+64], zmm2
vmovdqa32 zmm1, zmmword ptr [rax+63]
vmovdqa32 zmmword ptr [rax+63], zmm2
vmovdqa32 zmm1, zmmword ptr [rax-64]
vmovdqa32 zmmword ptr [rax-64], zmm2
vmovdqa32 zmm1, zmmword ptr [rax+8128]
vmovdqa32 zmmword ptr [rax+8128], zmm2
vmovdqa32 zmm1, zmmword ptr [rax+8192]
vmovdqa32 zmmword ptr [rax+8192], zmm2
vmovdqa32 zmm1, zmmword ptr [rbp]
vmovdqa32 zmmword ptr [rbp], zmm2
vmovdqa32 zmm1, zmmword ptr [r13]
vmovdqa32 zmmword ptr [r13], zmm2
vmovdqa32 zmm1, zmmword ptr [rsp+128]
vmovdqa32 zmmword ptr [rsp+128], zmm2
vmovdqa32 zmm1, zmmword ptr [r9+r15*4-512]
vmovdqa32 zmmword ptr [r9+r15*4-512], zmm2
vmovdqa32 zmm1, zmmword ptr [rip+sym]
vmovdqa32 zmmword ptr [rip+sym], zmm2
vmovdqa32 zmm20{k3}{z}, zmmword ptr [rax+64]
vmovdqa32 zmmword ptr [r9+rdx*8-128]{k6}, zmm31
vmovdqa64 xmm0, xmm1
vmovdqa64 xmm16, xmm31
vmovdqa64 xmm8{k1}{z}, xmm9
vmovdqa64 xmm8{k1}, xmm25
vmovdqa64 xmm1, xmmword ptr [rax]
vmovdqa64 xmmword ptr [rax], xmm2
vmovdqa64 xmm1, xmmword ptr [rax+16]
vmovdqa64 xmmword ptr [rax+16], xmm2
vmovdqa64 xmm1, xmmword ptr [rax+15]
vmovdqa64 xmmword ptr [rax+15], xmm2
vmovdqa64 xmm1, xmmword ptr [rax-16]
vmovdqa64 xmmword ptr [rax-16], xmm2
vmovdqa64 xmm1, xmmword ptr [rax+2032]
vmovdqa64 xmmword ptr [rax+2032], xmm2
vmovdqa64 xmm1, xmmword ptr [rax+2048]
vmovdqa64 xmmword ptr [rax+2048], xmm2
vmovdqa64 xmm1, xmmword ptr [rbp]
vmovdqa64 xmmword ptr [rbp], xmm2
vmovdqa64 xmm1, xmmword ptr [r13]
vmovdqa64 xmmword ptr [r13], xmm2
vmovdqa64 xmm1, xmmword ptr [rsp+32]
vmovdqa64 xmmword ptr [rsp+32], xmm2
vmovdqa64 xmm1, xmmword ptr [r9+r15*4-128]
vmovdqa64 xmmword ptr [r9+r15*4-128], xmm2
vmovdqa64 xmm1, xmmword ptr [rip+sym]
vmovdqa64 xmmword ptr [rip+sym], xmm2
vmovdqa64 xmm20{k3}{z}, xmmword ptr [rax+16]
vmovdqa64 xmmword ptr [r9+rdx*8-32]{k6}, xmm31
vmovdqa64 ymm0, ymm1
vmovdqa64 ymm16, ymm31
vmovdqa64 ymm8{k1}{z}, ymm9
vmovdqa64 ymm8{k1}, ymm25
vmovdqa64 ymm1, ymmword ptr [rax]
vmovdqa64 ymmword ptr [rax], ymm2
vmovdqa64 ymm1, ymmword ptr [rax+32]
vmovdqa64 ymmword ptr [rax+32], ymm2
vmovdqa64 ymm1, ymmword ptr [rax+31]
vmovdqa64 ymmword ptr [rax+31], ymm2
vmovdqa64 ymm1, ymmword ptr [rax-32]
vmovdqa64 ymmword ptr [rax-32], ymm2
vmovdqa64 ymm1, ymmword ptr [rax+4064]
vmovdqa64 ymmword ptr [rax+4064], ymm2
vmovdqa64 ymm1, ymmword ptr [rax+4096]
vmovdqa64 ymmword ptr [rax+4096], ymm2
vmovdqa64 ymm1, ymmword ptr [rbp]
vmovdqa64 ymmword ptr [rbp], ymm2
vmovdqa64 ymm1, ymmword ptr [r13]
vmovdqa64 ymmword ptr [r13], ymm2
vmovdqa64 ymm1, ymmword ptr [rsp+64]
vmovdqa64 ymmword ptr [rsp+64], ymm2
vmovdqa64 ymm1, ymmword ptr [r9+r15*4-256]
vmovdqa64 ymmword ptr [r9+r15*4-256], ymm2
vmovdqa64 ymm1, ymmword ptr [rip+sym]
vmovdqa64 ymmword ptr [rip+sym], ymm2
vmovdqa64 ymm20{k3}{z}, ymmword ptr [rax+32]
vmovdqa64 ymmword ptr [r9+rdx*8-64]{k6}, ymm31
vmovdqa64 zmm0, zmm1
vmovdqa64 zmm16, zmm31
vmovdqa64 zmm8{k1}{z}, zmm9
vmovdqa64 zmm8{k1}, zmm25
vmovdqa64 zmm1, zmmword ptr [rax]
vmovdqa64 zmmword ptr [rax], zmm2
vmovdqa64 zmm1, zmmword ptr [rax+64]
vmovdqa64 zmmword ptr [rax+64], zmm2
vmovdqa64 zmm1, zmmword ptr [rax+63]
vmovdqa64 zmmword ptr [rax+63], zmm2
vmovdqa64 zmm1, zmmword ptr [rax-64]
vmovdqa64 zmmword ptr [rax-64], zmm2
vmovdqa64 zmm1, zmmword ptr [rax+8128]
vmovdqa64 zmmword ptr [rax+8128], zmm2
vmovdqa64 zmm1, zmmword ptr [rax+8192]
vmovdqa64 zmmword ptr [rax+8192], zmm2
vmovdqa64 zmm1, zmmword ptr [rbp]
vmovdqa64 zmmword ptr [rbp], zmm2
vmovdqa64 zmm1, zmmword ptr [r13]
vmovdqa64 zmmword ptr [r13], zmm2
vmovdqa64 zmm1, zmmword ptr [rsp+128]
vmovdqa64 zmmword ptr [rsp+128], zmm2
vmovdqa64 zmm1, zmmword ptr [r9+r15*4-512]
vmovdqa64 zmmword ptr [r9+r15*4-512], zmm2
vmovdqa64 zmm1, zmmword ptr [rip+sym]
vmovdqa64 zmmword ptr [rip+sym], zmm2
vmovdqa64 zmm20{k3}{z}, zmmword ptr [rax+64]
vmovdqa64 zmmword ptr [r9+rdx*8-128]{k6}, zmm31
vmovdqu8 xmm0, xmm1
vmovdqu8 xmm16, xmm31
vmovdqu8 xmm8{k1}{z}, xmm9
vmovdqu8 xmm8{k1}, xmm25
vmovdqu8 xmm1, xmmword ptr [rax]
vmovdqu8 xmmword ptr [rax], xmm2
vmovdqu8 xmm1, xmmword ptr [rax+16]
vmovdqu8 xmmword ptr [rax+16], xmm2
vmovdqu8 xmm1, xmmword ptr [rax+15]
vmovdqu8 xmmword ptr [rax+15], xmm2
vmovdqu8 xmm1, xmmword ptr [rax-16]
vmovdqu8 xmmword ptr [rax-16], xmm2
vmovdqu8 xmm1, xmmword ptr [rax+2032]
vmovdqu8 xmmword ptr [rax+2032], xmm2
vmovdqu8 xmm1, xmmword ptr [rax+2048]
vmovdqu8 xmmword ptr [rax+2048], xmm2
vmovdqu8 xmm1, xmmword ptr [rbp]
vmovdqu8 xmmword ptr [rbp], xmm2
vmovdqu8 xmm1, xmmword ptr [r13]
vmovdqu8 xmmword ptr [r13], xmm2
vmovdqu8 xmm1, xmmword ptr [rsp+32]
vmovdqu8 xmmword ptr [rsp+32], xmm2
vmovdqu8 xmm1, xmmword ptr [r9+r15*4-128]
vmovdqu8 xmmword ptr [r9+r15*4-128], xmm2
vmovdqu8 xmm1, xmmword ptr [rip+sym]
vmovdqu8 xmmword ptr [rip+sym], xmm2
vmovdqu8 xmm20{k3}{z}, xmmword ptr [rax+16]
vmovdqu8 xmmword ptr [r9+rdx*8-32]{k6}, xmm31
vmovdqu8 ymm0, ymm1
vmovdqu8 ymm16, ymm31
vmovdqu8 ymm8{k1}{z}, ymm9
vmovdqu8 ymm8{k1}, ymm25
vmovdqu8 ymm1, ymmword ptr [rax]
vmovdqu8 ymmword ptr [rax], ymm2
vmovdqu8 ymm1, ymmword ptr [rax+32]
vmovdqu8 ymmword ptr [rax+32], ymm2
vmovdqu8 ymm1, ymmword ptr [rax+31]
vmovdqu8 ymmword ptr [rax+31], ymm2
vmovdqu8 ymm1, ymmword ptr [rax-32]
vmovdqu8 ymmword ptr [rax-32], ymm2
vmovdqu8 ymm1, ymmword ptr [rax+4064]
vmovdqu8 ymmword ptr [rax+4064], ymm2
vmovdqu8 ymm1, ymmword ptr [rax+4096]
vmovdqu8 ymmword ptr [rax+4096], ymm2
vmovdqu8 ymm1, ymmword ptr [rbp]
vmovdqu8 ymmword ptr [rbp], ymm2
vmovdqu8 ymm1, ymmword ptr [r13]
vmovdqu8 ymmword ptr [r13], ymm2
vmovdqu8 ymm1, ymmword ptr [rsp+64]
vmovdqu8 ymmword ptr [rsp+64], ymm2
vmovdqu8 ymm1, ymmword ptr [r9+r15*4-256]
vmovdqu8 ymmword ptr [r9+r15*4-256], ymm2
vmovdqu8 ymm1, ymmword ptr [rip+sym]
vmovdqu8 ymmword ptr [rip+sym], ymm2
vmovdqu8 ymm20{k3}{z}, ymmword ptr [rax+32]
vmovdqu8 ymmword ptr [r9+rdx*8-64]{k6}, ymm31
vmovdqu8 zmm0, zmm1
vmovdqu8 zmm16, zmm31
vmovdqu8 zmm8{k1}{z}, zmm9
vmovdqu8 zmm8{k1}, zmm25
vmovdqu8 zmm1, zmmword ptr [rax]
vmovdqu8 zmmword ptr [rax], zmm2
vmovdqu8 zmm1, zmmword ptr [rax+64]
vmovdqu8 zmmword ptr [rax+64], zmm2
vmovdqu8 zmm1, zmmword ptr [rax+63]
vmovdqu8 zmmword ptr [rax+63], zmm2
vmovdqu8 zmm1, zmmword ptr [rax-64]
vmovdqu8 zmmword ptr [rax-64], zmm2
vmovdqu8 zmm1, zmmword ptr [rax+8128]
vmovdqu8 zmmword ptr [rax+8128], zmm2
vmovdqu8 zmm1, zmmword ptr [rax+8192]
vmovdqu8 zmmword ptr [rax+8192], zmm2
vmovdqu8 zmm1, zmmword ptr [rbp]
vmovdqu8 zmmword ptr [rbp], zmm2
vmovdqu8 zmm1, zmmword ptr [r13]
vmovdqu8 zmmword ptr [r13], zmm2
vmovdqu8 zmm1, zmmword ptr [rsp+128]
vmovdqu8 zmmword ptr [rsp+128], zmm2
vmovdqu8 zmm1, zmmword ptr [r9+r15*4-512]
vmovdqu8 zmmword ptr [r9+r15*4-512], zmm2
vmovdqu8 zmm1, zmmword ptr [rip+sym]
vmovdqu8 zmmword ptr [rip+sym], zmm2
vmovdqu8 zmm20{k3}{z}, zmmword ptr [rax+64]
vmovdqu8 zmmword ptr [r9+rdx*8-128]{k6}, zmm31
vmovdqu16 xmm0, xmm1
vmovdqu16 xmm16, xmm31
vmovdqu16 xmm8{k1}{z}, xmm9
vmovdqu16 xmm8{k1}, xmm25
vmovdqu16 xmm1, xmmword ptr [rax]
vmovdqu16 xmmword ptr [rax], xmm2
vmovdqu16 xmm1, xmmword ptr [rax+16]
vmovdqu16 xmmword ptr [rax+16], xmm2
vmovdqu16 xmm1, xmmword ptr [rax+15]
vmovdqu16 xmmword ptr [rax+15], xmm2
vmovdqu16 xmm1, xmmword ptr [rax-16]
vmovdqu16 xmmword ptr [rax-16], xmm2
vmovdqu16 xmm1, xmmword ptr [rax+2032]
vmovdqu16 xmmword ptr [rax+2032], xmm2
vmovdqu16 xmm1, xmmword ptr [rax+2048]
vmovdqu16 xmmword ptr [rax+2048], xmm2
vmovdqu16 xmm1, xmmword ptr [rbp]
vmovdqu16 xmmword ptr [rbp], xmm2
vmovdqu16 xmm1, xmmword ptr [r13]
vmovdqu16 xmmword ptr [r13], xmm2
vmovdqu16 xmm1, xmmword ptr [rsp+32]
vmovdqu16 xmmword ptr [rsp+32], xmm2
vmovdqu16 xmm1, xmmword ptr [r9+r15*4-128]
vmovdqu16 xmmword ptr [r9+r15*4-128], xmm2
vmovdqu16 xmm1, xmmword ptr [rip+sym]
vmovdqu16 xmmword ptr [rip+sym], xmm2
vmovdqu16 xmm20{k3}{z}, xmmword ptr [rax+16]
vmovdqu16 xmmword ptr [r9+rdx*8-32]{k6}, xmm31
vmovdqu16 ymm0, ymm1
vmovdqu16 ymm16, ymm31
vmovdqu16 ymm8{k1}{z}, ymm9
vmovdqu16 ymm8{k1}, ymm25
vmovdqu16 ymm1, ymmword ptr [rax]
vmovdqu16 ymmword ptr [rax], ymm2
vmovdqu16 ymm1, ymmword ptr [rax+32]
vmovdqu16 ymmword ptr [rax+32], ymm2
vmovdqu16 ymm1, ymmword ptr [rax+31]
vmovdqu16 ymmword ptr [rax+31], ymm2
vmovdqu16 ymm1, ymmword ptr [rax-32]
vmovdqu16 ymmword ptr [rax-32], ymm2
vmovdqu16 ymm1, ymmword ptr [rax+4064]
vmovdqu16 ymmword ptr [rax+4064], ymm2
vmovdqu16 ymm1, ymmword ptr [rax+4096]
vmovdqu16 ymmword ptr [rax+4096], ymm2
vmovdqu16 ymm1, ymmword ptr [rbp]
vmovdqu16 ymmword ptr [rbp], ymm2
vmovdqu16 ymm1, ymmword ptr [r13]
vmovdqu16 ymmword ptr [r13], ymm2
vmovdqu16 ymm1, ymmword ptr [rsp+64]
vmovdqu16 ymmword ptr [rsp+64], ymm2
vmovdqu16 ymm1, ymmword ptr [r9+r15*4-256]
vmovdqu16 ymmword ptr [r9+r15*4-256], ymm2
vmovdqu16 ymm1, ymmword ptr [rip+sym]
vmovdqu16 ymmword ptr [rip+sym], ymm2
vmovdqu16 ymm20{k3}{z}, ymmword ptr [rax+32]
vmovdqu16 ymmword ptr [r9+rdx*8-64]{k6}, ymm31
vmovdqu16 zmm0, zmm1
vmovdqu16 zmm16, zmm31
vmovdqu16 zmm8{k1}{z}, zmm9
vmovdqu16 zmm8{k1}, zmm25
vmovdqu16 zmm1, zmmword ptr [rax]
vmovdqu16 zmmword ptr [rax], zmm2
vmovdqu16 zmm1, zmmword ptr [rax+64]
vmovdqu16 zmmword ptr [rax+64], zmm2
vmovdqu16 zmm1, zmmword ptr [rax+63]
vmovdqu16 zmmword ptr [rax+63], zmm2
vmovdqu16 zmm1, zmmword ptr [rax-64]
vmovdqu16 zmmword ptr [rax-64], zmm2
vmovdqu16 zmm1, zmmword ptr [rax+8128]
vmovdqu16 zmmword ptr [rax+8128], zmm2
vmovdqu16 zmm1, zmmword ptr [rax+8192]
vmovdqu16 zmmword ptr [rax+8192], zmm2
vmovdqu16 zmm1, zmmword ptr [rbp]
vmovdqu16 zmmword ptr [rbp], zmm2
vmovdqu16 zmm1, zmmword ptr [r13]
vmovdqu16 zmmword ptr [r13], zmm2
vmovdqu16 zmm1, zmmword ptr [rsp+128]
vmovdqu16 zmmword ptr [rsp+128], zmm2
vmovdqu16 zmm1, zmmword ptr [r9+r15*4-512]
vmovdqu16 zmmword ptr [r9+r15*4-512], zmm2
vmovdqu16 zmm1, zmmword ptr [rip+sym]
vmovdqu16 zmmword ptr [rip+sym], zmm2
vmovdqu16 zmm20{k3}{z}, zmmword ptr [rax+64]
vmovdqu16 zmmword ptr [r9+rdx*8-128]{k6}, zmm31
vmovdqu32 xmm0, xmm1
vmovdqu32 xmm16, xmm31
vmovdqu32 xmm8{k1}{z}, xmm9
vmovdqu32 xmm8{k1}, xmm25
vmovdqu32 xmm1, xmmword ptr [rax]
vmovdqu32 xmmword ptr [rax], xmm2
vmovdqu32 xmm1, xmmword ptr [rax+16]
vmovdqu32 xmmword ptr [rax+16], xmm2
vmovdqu32 xmm1, xmmword ptr [rax+15]
vmovdqu32 xmmword ptr [rax+15], xmm2
vmovdqu32 xmm1, xmmword ptr [rax-16]
vmovdqu32 xmmword ptr [rax-16], xmm2
vmovdqu32 xmm1, xmmword ptr [rax+2032]
vmovdqu32 xmmword ptr [rax+2032], xmm2
vmovdqu32 xmm1, xmmword ptr [rax+2048]
vmovdqu32 xmmword ptr [rax+2048], xmm2
vmovdqu32 xmm1, xmmword ptr [rbp]
vmovdqu32 xmmword ptr [rbp], xmm2
vmovdqu32 xmm1, xmmword ptr [r13]
vmovdqu32 xmmword ptr [r13], xmm2
vmovdqu32 xmm1, xmmword ptr [rsp+32]
vmovdqu32 xmmword ptr [rsp+32], xmm2
vmovdqu32 xmm1, xmmword ptr [r9+r15*4-128]
vmovdqu32 xmmword ptr [r9+r15*4-128], xmm2
vmovdqu32 xmm1, xmmword ptr [rip+sym]
vmovdqu32 xmmword ptr [rip+sym], xmm2
vmovdqu32 xmm20{k3}{z}, xmmword ptr [rax+16]
vmovdqu32 xmmword ptr [r9+rdx*8-32]{k6}, xmm31
vmovdqu32 ymm0, ymm1
vmovdqu32 ymm16, ymm31
vmovdqu32 ymm8{k1}{z}, ymm9
vmovdqu32 ymm8{k1}, ymm25
vmovdqu32 ymm1, ymmword ptr [rax]
vmovdqu32 ymmword ptr [rax], ymm2
vmovdqu32 ymm1, ymmword ptr [rax+32]
vmovdqu32 ymmword ptr [rax+32], ymm2
vmovdqu32 ymm1, ymmword ptr [rax+31]
vmovdqu32 ymmword ptr [rax+31], ymm2
vmovdqu32 ymm1, ymmword ptr [rax-32]
vmovdqu32 ymmword ptr [rax-32], ymm2
vmovdqu32 ymm1, ymmword ptr [rax+4064]
vmovdqu32 ymmword ptr [rax+4064], ymm2
vmovdqu32 ymm1, ymmword ptr [rax+4096]
vmovdqu32 ymmword ptr [rax+4096], ymm2
vmovdqu32 ymm1, ymmword ptr [rbp]
vmovdqu32 ymmword ptr [rbp], ymm2
vmovdqu32 ymm1, ymmword ptr [r13]
vmovdqu32 ymmword ptr [r13], ymm2
vmovdqu32 ymm1, ymmword ptr [rsp+64]
vmovdqu32 ymmword ptr [rsp+64], ymm2
vmovdqu32 ymm1, ymmword ptr [r9+r15*4-256]
vmovdqu32 ymmword ptr [r9+r15*4-256], ymm2
vmovdqu32 ymm1, ymmword ptr [rip+sym]
vmovdqu32 ymmword ptr [rip+sym], ymm2
vmovdqu32 ymm20{k3}{z}, ymmword ptr [rax+32]
vmovdqu32 ymmword ptr [r9+rdx*8-64]{k6}, ymm31
vmovdqu32 zmm0, zmm1
vmovdqu32 zmm16, zmm31
vmovdqu32 zmm8{k1}{z}, zmm9
vmovdqu32 zmm8{k1}, zmm25
vmovdqu32 zmm1, zmmword ptr [rax]
vmovdqu32 zmmword ptr [rax], zmm2
vmovdqu32 zmm1, zmmword ptr [rax+64]
vmovdqu32 zmmword ptr [rax+64], zmm2
vmovdqu32 zmm1, zmmword ptr [rax+63]
vmovdqu32 zmmword ptr [rax+63], zmm2
vmovdqu32 zmm1, zmmword ptr [rax-64]
vmovdqu32 zmmword ptr [rax-64], zmm2
vmovdqu32 zmm1, zmmword ptr [rax+8128]
vmovdqu32 zmmword ptr [rax+8128], zmm2
vmovdqu32 zmm1, zmmword ptr [rax+8192]
vmovdqu32 zmmword ptr [rax+8192], zmm2
vmovdqu32 zmm1, zmmword ptr [rbp]
vmovdqu32 zmmword ptr [rbp], zmm2
vmovdqu32 zmm1, zmmword ptr [r13]
vmovdqu32 zmmword ptr [r13], zmm2
vmovdqu32 zmm1, zmmword ptr [rsp+128]
vmovdqu32 zmmword ptr [rsp+128], zmm2
vmovdqu32 zmm1, zmmword ptr [r9+r15*4-512]
vmovdqu32 zmmword ptr [r9+r15*4-512], zmm2
vmovdqu32 zmm1, zmmword ptr [rip+sym]
vmovdqu32 zmmword ptr [rip+sym], zmm2
vmovdqu32 zmm20{k3}{z}, zmmword ptr [rax+64]
vmovdqu32 zmmword ptr [r9+rdx*8-128]{k6}, zmm31
vmovdqu64 xmm0, xmm1
vmovdqu64 xmm16, xmm31
vmovdqu64 xmm8{k1}{z}, xmm9
vmovdqu64 xmm8{k1}, xmm25
vmovdqu64 xmm1, xmmword ptr [rax]
vmovdqu64 xmmword ptr [rax], xmm2
vmovdqu64 xmm1, xmmword ptr [rax+16]
vmovdqu64 xmmword ptr [rax+16], xmm2
vmovdqu64 xmm1, xmmword ptr [rax+15]
vmovdqu64 xmmword ptr [rax+15], xmm2
vmovdqu64 xmm1, xmmword ptr [rax-16]
vmovdqu64 xmmword ptr [rax-16], xmm2
vmovdqu64 xmm1, xmmword ptr [rax+2032]
vmovdqu64 xmmword ptr [rax+2032], xmm2
vmovdqu64 xmm1, xmmword ptr [rax+2048]
vmovdqu64 xmmword ptr [rax+2048], xmm2
vmovdqu64 xmm1, xmmword ptr [rbp]
vmovdqu64 xmmword ptr [rbp], xmm2
vmovdqu64 xmm1, xmmword ptr [r13]
vmovdqu64 xmmword ptr [r13], xmm2
vmovdqu64 xmm1, xmmword ptr [rsp+32]
vmovdqu64 xmmword ptr [rsp+32], xmm2
vmovdqu64 xmm1, xmmword ptr [r9+r15*4-128]
vmovdqu64 xmmword ptr [r9+r15*4-128], xmm2
vmovdqu64 xmm1, xmmword ptr [rip+sym]
vmovdqu64 xmmword ptr [rip+sym], xmm2
vmovdqu64 xmm20{k3}{z}, xmmword ptr [rax+16]
vmovdqu64 xmmword ptr [r9+rdx*8-32]{k6}, xmm31
vmovdqu64 ymm0, ymm1
vmovdqu64 ymm16, ymm31
vmovdqu64 ymm8{k1}{z}, ymm9
vmovdqu64 ymm8{k1}, ymm25
vmovdqu64 ymm1, ymmword ptr [rax]
vmovdqu64 ymmword ptr [rax], ymm2
vmovdqu64 ymm1, ymmword ptr [rax+32]
vmovdqu64 ymmword ptr [rax+32], ymm2
vmovdqu64 ymm1, ymmword ptr [rax+31]
vmovdqu64 ymmword ptr [rax+31], ymm2
vmovdqu64 ymm1, ymmword ptr [rax-32]
vmovdqu64 ymmword ptr [rax-32], ymm2
vmovdqu64 ymm1, ymmword ptr [rax+4064]
vmovdqu64 ymmword ptr [rax+4064], ymm2
vmovdqu64 ymm1, ymmword ptr [rax+4096]
vmovdqu64 ymmword ptr [rax+4096], ymm2
vmovdqu64 ymm1, ymmword ptr [rbp]
vmovdqu64 ymmword ptr [rbp], ymm2
vmovdqu64 ymm1, ymmword ptr [r13]
vmovdqu64 ymmword ptr [r13], ymm2
vmovdqu64 ymm1, ymmword ptr [rsp+64]
vmovdqu64 ymmword ptr [rsp+64], ymm2
vmovdqu64 ymm1, ymmword ptr [r9+r15*4-256]
vmovdqu64 ymmword ptr [r9+r15*4-256], ymm2
vmovdqu64 ymm1, ymmword ptr [rip+sym]
vmovdqu64 ymmword ptr [rip+sym], ymm2
vmovdqu64 ymm20{k3}{z}, ymmword ptr [rax+32]
vmovdqu64 ymmword ptr [r9+rdx*8-64]{k6}, ymm31
vmovdqu64 zmm0, zmm1
vmovdqu64 zmm16, zmm31
vmovdqu64 zmm8{k1}{z}, zmm9
vmovdqu64 zmm8{k1}, zmm25
vmovdqu64 zmm1, zmmword ptr [rax]
vmovdqu64 zmmword ptr [rax], zmm2
vmovdqu64 zmm1, zmmword ptr [rax+64]
vmovdqu64 zmmword ptr [rax+64], zmm2
vmovdqu64 zmm1, zmmword ptr [rax+63]
vmovdqu64 zmmword ptr [rax+63], zmm2
vmovdqu64 zmm1, zmmword ptr [rax-64]
vmovdqu64 zmmword ptr [rax-64], zmm2
vmovdqu64 zmm1, zmmword ptr [rax+8128]
vmovdqu64 zmmword ptr [rax+8128], zmm2
vmovdqu64 zmm1, zmmword ptr [rax+8192]
vmovdqu64 zmmword ptr [rax+8192], zmm2
vmovdqu64 zmm1, zmmword ptr [rbp]
vmovdqu64 zmmword ptr [rbp], zmm2
vmovdqu64 zmm1, zmmword ptr [r13]
vmovdqu64 zmmword ptr [r13], zmm2
vmovdqu64 zmm1, zmmword ptr [rsp+128]
vmovdqu64 zmmword ptr [rsp+128], zmm2
vmovdqu64 zmm1, zmmword ptr [r9+r15*4-512]
vmovdqu64 zmmword ptr [r9+r15*4-512], zmm2
vmovdqu64 zmm1, zmmword ptr [rip+sym]
vmovdqu64 zmmword ptr [rip+sym], zmm2
vmovdqu64 zmm20{k3}{z}, zmmword ptr [rax+64]
vmovdqu64 zmmword ptr [r9+rdx*8-128]{k6}, zmm31
# mask and vector conversions
vpmovm2b xmm0, k1
vpmovm2b xmm31, k7
vpmovb2m k1, xmm0
vpmovb2m k7, xmm31
vpmovb2m k2, xmm9
vpmovm2b ymm0, k1
vpmovm2b ymm31, k7
vpmovb2m k1, ymm0
vpmovb2m k7, ymm31
vpmovb2m k2, ymm9
vpmovm2b zmm0, k1
vpmovm2b zmm31, k7
vpmovb2m k1, zmm0
vpmovb2m k7, zmm31
vpmovb2m k2, zmm9
vpmovm2w xmm0, k1
vpmovm2w xmm31, k7
vpmovw2m k1, xmm0
vpmovw2m k7, xmm31
vpmovw2m k2, xmm9
vpmovm2w ymm0, k1
vpmovm2w ymm31, k7
vpmovw2m k1, ymm0
vpmovw2m k7, ymm31
vpmovw2m k2, ymm9
vpmovm2w zmm0, k1
vpmovm2w zmm31, k7
vpmovw2m k1, zmm0
vpmovw2m k7, zmm31
vpmovw2m k2, zmm9
vpmovm2d xmm0, k1
vpmovm2d xmm31, k7
vpmovd2m k1, xmm0
vpmovd2m k7, xmm31
vpmovd2m k2, xmm9
vpmovm2d ymm0, k1
vpmovm2d ymm31, k7
vpmovd2m k1, ymm0
vpmovd2m k7, ymm31
vpmovd2m k2, ymm9
vpmovm2d zmm0, k1
vpmovm2d zmm31, k7
vpmovd2m k1, zmm0
vpmovd2m k7, zmm31
vpmovd2m k2, zmm9
vpmovm2q xmm0, k1
vpmovm2q xmm31, k7
vpmovq2m k1, xmm0
vpmovq2m k7, xmm31
vpmovq2m k2, xmm9
vpmovm2q ymm0, k1
vpmovm2q ymm31, k7
vpmovq2m k1, ymm0
vpmovq2m k7, ymm31
vpmovq2m k2, ymm9
vpmovm2q zmm0, k1
vpmovm2q zmm31, k7
vpmovq2m k1, zmm0
vpmovq2m k7, zmm31
vpmovq2m k2, zmm9
# broadcasts
vpbroadcastb xmm0, xmm1
vpbroadcastb xmm16, xmm31
vpbroadcastb xmm8{k1}{z}, xmm9
vpbroadcastb xmm1, byte ptr [rax+0]
vpbroadcastb xmm1, byte ptr [rax+1]
vpbroadcastb xmm1, byte ptr [rax+0]
vpbroadcastb xmm1, byte ptr [rax+127]
vpbroadcastb xmm1, byte ptr [rax+128]
vpbroadcastb xmm1, byte ptr [rax-128]
vpbroadcastb xmm1, byte ptr [rax-129]
vpbroadcastb xmm20{k5}, byte ptr [r9+rcx*4+1]
vpbroadcastb xmm0, eax
vpbroadcastb xmm31{k1}{z}, eax
vpbroadcastb xmm0, r9d
vpbroadcastb xmm31{k1}{z}, r9d
vpbroadcastb xmm0, r15d
vpbroadcastb xmm31{k1}{z}, r15d
vpbroadcastb xmm0, ebp
vpbroadcastb xmm31{k1}{z}, ebp
vpbroadcastb ymm0, xmm1
vpbroadcastb ymm16, xmm31
vpbroadcastb ymm8{k1}{z}, xmm9
vpbroadcastb ymm1, byte ptr [rax+0]
vpbroadcastb ymm1, byte ptr [rax+1]
vpbroadcastb ymm1, byte ptr [rax+0]
vpbroadcastb ymm1, byte ptr [rax+127]
vpbroadcastb ymm1, byte ptr [rax+128]
vpbroadcastb ymm1, byte ptr [rax-128]
vpbroadcastb ymm1, byte ptr [rax-129]
vpbroadcastb ymm20{k5}, byte ptr [r9+rcx*4+1]
vpbroadcastb ymm0, eax
vpbroadcastb ymm31{k1}{z}, eax
vpbroadcastb ymm0, r9d
vpbroadcastb ymm31{k1}{z}, r9d
vpbroadcastb ymm0, r15d
vpbroadcastb ymm31{k1}{z}, r15d
vpbroadcastb ymm0, ebp
vpbroadcastb ymm31{k1}{z}, ebp
vpbroadcastb zmm0, xmm1
vpbroadcastb zmm16, xmm31
vpbroadcastb zmm8{k1}{z}, xmm9
vpbroadcastb zmm1, byte ptr [rax+0]
vpbroadcastb zmm1, byte ptr [rax+1]
vpbroadcastb zmm1, byte ptr [rax+0]
vpbroadcastb zmm1, byte ptr [rax+127]
vpbroadcastb zmm1, byte ptr [rax+128]
vpbroadcastb zmm1, byte ptr [rax-128]
vpbroadcastb zmm1, byte ptr [rax-129]
vpbroadcastb zmm20{k5}, byte ptr [r9+rcx*4+1]
vpbroadcastb zmm0, eax
vpbroadcastb zmm31{k1}{z}, eax
vpbroadcastb zmm0, r9d
vpbroadcastb zmm31{k1}{z}, r9d
vpbroadcastb zmm0, r15d
vpbroadcastb zmm31{k1}{z}, r15d
vpbroadcastb zmm0, ebp
vpbroadcastb zmm31{k1}{z}, ebp
vpbroadcastw xmm0, xmm1
vpbroadcastw xmm16, xmm31
vpbroadcastw xmm8{k1}{z}, xmm9
vpbroadcastw xmm1, word ptr [rax+0]
vpbroadcastw xmm1, word ptr [rax+2]
vpbroadcastw xmm1, word ptr [rax+1]
vpbroadcastw xmm1, word ptr [rax+254]
vpbroadcastw xmm1, word ptr [rax+256]
vpbroadcastw xmm1, word ptr [rax-256]
vpbroadcastw xmm1, word ptr [rax-258]
vpbroadcastw xmm20{k5}, word ptr [r9+rcx*4+2]
vpbroadcastw xmm0, eax
vpbroadcastw xmm31{k1}{z}, eax
vpbroadcastw xmm0, r9d
vpbroadcastw xmm31{k1}{z}, r9d
vpbroadcastw xmm0, r15d
vpbroadcastw xmm31{k1}{z}, r15d
vpbroadcastw xmm0, ebp
vpbroadcastw xmm31{k1}{z}, ebp
vpbroadcastw ymm0, xmm1
vpbroadcastw ymm16, xmm31
vpbroadcastw ymm8{k1}{z}, xmm9
vpbroadcastw ymm1, word ptr [rax+0]
vpbroadcastw ymm1, word ptr [rax+2]
vpbroadcastw ymm1, word ptr [rax+1]
vpbroadcastw ymm1, word ptr [rax+254]
vpbroadcastw ymm1, word ptr [rax+256]
vpbroadcastw ymm1, word ptr [rax-256]
vpbroadcastw ymm1, word ptr [rax-258]
vpbroadcastw ymm20{k5}, word ptr [r9+rcx*4+2]
vpbroadcastw ymm0, eax
vpbroadcastw ymm31{k1}{z}, eax
vpbroadcastw ymm0, r9d
vpbroadcastw ymm31{k1}{z}, r9d
vpbroadcastw ymm0, r15d
vpbroadcastw ymm31{k1}{z}, r15d
vpbroadcastw ymm0, ebp
vpbroadcastw ymm31{k1}{z}, ebp
vpbroadcastw zmm0, xmm1
vpbroadcastw zmm16, xmm31
vpbroadcastw zmm8{k1}{z}, xmm9
vpbroadcastw zmm1, word ptr [rax+0]
vpbroadcastw zmm1, word ptr [rax+2]
vpbroadcastw zmm1, word ptr [rax+1]
vpbroadcastw zmm1, word ptr [rax+254]
vpbroadcastw zmm1, word ptr [rax+256]
vpbroadcastw zmm1, word ptr [rax-256]
vpbroadcastw zmm1, word ptr [rax-258]
vpbroadcastw zmm20{k5}, word ptr [r9+rcx*4+2]
vpbroadcastw zmm0, eax
vpbroadcastw zmm31{k1}{z}, eax
vpbroadcastw zmm0, r9d
vpbroadcastw zmm31{k1}{z}, r9d
vpbroadcastw zmm0, r15d
vpbroadcastw zmm31{k1}{z}, r15d
vpbroadcastw zmm0, ebp
vpbroadcastw zmm31{k1}{z}, ebp
vpbroadcastd xmm0, xmm1
vpbroadcastd xmm16, xmm31
vpbroadcastd xmm8{k1}{z}, xmm9
vpbroadcastd xmm1, dword ptr [rax+0]
vpbroadcastd xmm1, dword ptr [rax+4]
vpbroadcastd xmm1, dword ptr [rax+3]
vpbroadcastd xmm1, dword ptr [rax+508]
vpbroadcastd xmm1, dword ptr [rax+512]
vpbroadcastd xmm1, dword ptr [rax-512]
vpbroadcastd xmm1, dword ptr [rax-516]
vpbroadcastd xmm20{k5}, dword ptr [r9+rcx*4+4]
vpbroadcastd xmm0, eax
vpbroadcastd xmm31{k1}{z}, eax
vpbroadcastd xmm0, r9d
vpbroadcastd xmm31{k1}{z}, r9d
vpbroadcastd xmm0, r15d
vpbroadcastd xmm31{k1}{z}, r15d
vpbroadcastd xmm0, ebp
vpbroadcastd xmm31{k1}{z}, ebp
vpbroadcastd ymm0, xmm1
vpbroadcastd ymm16, xmm31
vpbroadcastd ymm8{k1}{z}, xmm9
vpbroadcastd ymm1, dword ptr [rax+0]
vpbroadcastd ymm1, dword ptr [rax+4]
vpbroadcastd ymm1, dword ptr [rax+3]
vpbroadcastd ymm1, dword ptr [rax+508]
vpbroadcastd ymm1, dword ptr [rax+512]
vpbroadcastd ymm1, dword ptr [rax-512]
vpbroadcastd ymm1, dword ptr [rax-516]
vpbroadcastd ymm20{k5}, dword ptr [r9+rcx*4+4]
vpbroadcastd ymm0, eax
vpbroadcastd ymm31{k1}{z}, eax
vpbroadcastd ymm0, r9d
vpbroadcastd ymm31{k1}{z}, r9d
vpbroadcastd ymm0, r15d
vpbroadcastd ymm31{k1}{z}, r15d
vpbroadcastd ymm0, ebp
vpbroadcastd ymm31{k1}{z}, ebp
vpbroadcastd zmm0, xmm1
vpbroadcastd zmm16, xmm31
vpbroadcastd zmm8{k1}{z}, xmm9
vpbroadcastd zmm1, dword ptr [rax+0]
vpbroadcastd zmm1, dword ptr [rax+4]
vpbroadcastd zmm1, dword ptr [rax+3]
vpbroadcastd zmm1, dword ptr [rax+508]
vpbroadcastd zmm1, dword ptr [rax+512]
vpbroadcastd zmm1, dword ptr [rax-512]
vpbroadcastd zmm1, dword ptr [rax-516]
vpbroadcastd zmm20{k5}, dword ptr [r9+rcx*4+4]
vpbroadcastd zmm0, eax
vpbroadcastd zmm31{k1}{z}, eax
vpbroadcastd zmm0, r9d
vpbroadcastd zmm31{k1}{z}, r9d
vpbroadcastd zmm0, r15d
vpbroadcastd zmm31{k1}{z}, r15d
vpbroadcastd zmm0, ebp
vpbroadcastd zmm31{k1}{z}, ebp
vpbroadcastq xmm0, xmm1
vpbroadcastq xmm16, xmm31
vpbroadcastq xmm8{k1}{z}, xmm9
vpbroadcastq xmm1, qword ptr [rax+0]
vpbroadcastq xmm1, qword ptr [rax+8]
vpbroadcastq xmm1, qword ptr [rax+7]
vpbroadcastq xmm1, qword ptr [rax+1016]
vpbroadcastq xmm1, qword ptr [rax+1024]
vpbroadcastq xmm1, qword ptr [rax-1024]
vpbroadcastq xmm1, qword ptr [rax-1032]
vpbroadcastq xmm20{k5}, qword ptr [r9+rcx*4+8]
vpbroadcastq xmm0, rax
vpbroadcastq xmm31{k1}{z}, rax
vpbroadcastq xmm0, r9
vpbroadcastq xmm31{k1}{z}, r9
vpbroadcastq xmm0, r15
vpbroadcastq xmm31{k1}{z}, r15
vpbroadcastq xmm0, rbp
vpbroadcastq xmm31{k1}{z}, rbp
vpbroadcastq ymm0, xmm1
vpbroadcastq ymm16, xmm31
vpbroadcastq ymm8{k1}{z}, xmm9
vpbroadcastq ymm1, qword ptr [rax+0]
vpbroadcastq ymm1, qword ptr [rax+8]
vpbroadcastq ymm1, qword ptr [rax+7]
vpbroadcastq ymm1, qword ptr [rax+1016]
vpbroadcastq ymm1, qword ptr [rax+1024]
vpbroadcastq ymm1, qword ptr [rax-1024]
vpbroadcastq ymm1, qword ptr [rax-1032]
vpbroadcastq ymm20{k5}, qword ptr [r9+rcx*4+8]
vpbroadcastq ymm0, rax
vpbroadcastq ymm31{k1}{z}, rax
vpbroadcastq ymm0, r9
vpbroadcastq ymm31{k1}{z}, r9
vpbroadcastq ymm0, r15
vpbroadcastq ymm31{k1}{z}, r15
vpbroadcastq ymm0, rbp
vpbroadcastq ymm31{k1}{z}, rbp
vpbroadcastq zmm0, xmm1
vpbroadcastq zmm16, xmm31
vpbroadcastq zmm8{k1}{z}, xmm9
vpbroadcastq zmm1, qword ptr [rax+0]
vpbroadcastq zmm1, qword ptr [rax+8]
vpbroadcastq zmm1, qword ptr [rax+7]
vpbroadcastq zmm1, qword ptr [rax+1016]
vpbroadcastq zmm1, qword ptr [rax+1024]
vpbroadcastq zmm1, qword ptr [rax-1024]
vpbroadcastq zmm1, qword ptr [rax-1032]
vpbroadcastq zmm20{k5}, qword ptr [r9+rcx*4+8]
vpbroadcastq zmm0, rax
vpbroadcastq zmm31{k1}{z}, rax
vpbroadcastq zmm0, r9
vpbroadcastq zmm31{k1}{z}, r9
vpbroadcastq zmm0, r15
vpbroadcastq zmm31{k1}{z}, r15
vpbroadcastq zmm0, rbp
vpbroadcastq zmm31{k1}{z}, rbp
# mask registers
kmovb k1, k2
kmovb k7, k0
kmovb k1, byte ptr [rax]
kmovb k1, byte ptr [r9+rcx*2+16]
kmovb byte ptr [rax+rbx*8], k3
kmovb byte ptr [r13], k7
kmovb k1, eax
kmovb k7, eax
kmovb eax, k1
kmovb eax, k7
kmovb k1, r15d
kmovb k7, r15d
kmovb r15d, k1
kmovb r15d, k7
kandb k1, k2, k3
kandb k7, k6, k5
kandnb k1, k2, k3
kandnb k7, k6, k5
korb k1, k2, k3
korb k7, k6, k5
kxnorb k1, k2, k3
kxnorb k7, k6, k5
kxorb k1, k2, k3
kxorb k7, k6, k5
knotb k1, k2
knotb k7, k0
kortestb k1, k2
kortestb k7, k0
ktestb k1, k2
ktestb k7, k0
kmovw k1, k2
kmovw k7, k0
kmovw k1, word ptr [rax]
kmovw k1, word ptr [r9+rcx*2+16]
kmovw word ptr [rax+rbx*8], k3
kmovw word ptr [r13], k7
kmovw k1, eax
kmovw k7, eax
kmovw eax, k1
kmovw eax, k7
kmovw k1, r15d
kmovw k7, r15d
kmovw r15d, k1
kmovw r15d, k7
kandw k1, k2, k3
kandw k7, k6, k5
kandnw k1, k2, k3
kandnw k7, k6, k5
korw k1, k2, k3
korw k7, k6, k5
kxnorw k1, k2, k3
kxnorw k7, k6, k5
kxorw k1, k2, k3
kxorw k7, k6, k5
knotw k1, k2
knotw k7, k0
kortestw k1, k2
kortestw k7, k0
ktestw k1, k2
ktestw k7, k0
kmovd k1, k2
kmovd k7, k0
kmovd k1, dword ptr [rax]
kmovd k1, dword ptr [r9+rcx*2+16]
kmovd dword ptr [rax+rbx*8], k3
kmovd dword ptr [r13], k7
kmovd k1, eax
kmovd k7, eax
kmovd eax, k1
kmovd eax, k7
kmovd k1, r15d
kmovd k7, r15d
kmovd r15d, k1
kmovd r15d, k7
kandd k1, k2, k3
kandd k7, k6, k5
kandnd k1, k2, k3
kandnd k7, k6, k5
kord k1, k2, k3
kord k7, k6, k5
kxnord k1, k2, k3
kxnord k7, k6, k5
kxord k1, k2, k3
kxord k7, k6, k5
knotd k1, k2
knotd k7, k0
kortestd k1, k2
kortestd k7, k0
ktestd k1, k2
ktestd k7, k0
kmovq k1, k2
kmovq k7, k0
kmovq k1, qword ptr [rax]
kmovq k1, qword ptr [r9+rcx*2+16]
kmovq qword ptr [rax+rbx*8], k3
kmovq qword ptr [r13], k7
kmovq k1, rax
kmovq k7, rax
kmovq rax, k1
kmovq rax, k7
kmovq k1, r15
kmovq k7, r15
kmovq r15, k1
kmovq r15, k7
kandq k1, k2, k3
kandq k7, k6, k5
kandnq k1, k2, k3
kandnq k7, k6, k5
korq k1, k2, k3
korq k7, k6, k5
kxnorq k1, k2, k3
kxnorq k7, k6, k5
kxorq k1, k2, k3
kxorq k7, k6, k5
knotq k1, k2
knotq k7, k0
kortestq k1, k2
kortestq k7, k0
ktestq k1, k2
ktestq k7, k0
# forms that stay VEX
vpaddd xmm0, xmm1, xmm2
vpaddd ymm8, ymm9, ymm10
vaddps ymm0, ymm1, ymmword ptr [rax+64]
vpcmpeqb ymm0, ymm1, ymm2
vpbroadcastb ymm0, xmm1
vpsllq ymm1, ymm2, xmm3
vcmpps ymm0, ymm1, ymm2, 1
vmovdqu ymm0, ymmword ptr [rax]
sym:
