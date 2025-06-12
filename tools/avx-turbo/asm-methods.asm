BITS 64
default rel

%if (__NASM_MAJOR__ < 2) || (__NASM_MINOR__ < 11)
%deftok ver __NASM_VER__
%error Your nasm version (ver) is too old, you need at least 2.11 to compile this
%endif

%include "nasm-utils-inc.asm"

nasm_util_assert_boilerplate
thunk_boilerplate

; aligns and declares the global label for the bench with the given name
; also potentally checks the ABI compliance (if enabled)
%macro define_func 1
abi_checked_function %1
%endmacro

; define a test func that unrolls the loop by 100
; with the given body instruction
; %1 - function name
; %2 - init instruction (e.g., xor out the variable you'll add to)
; %3 - loop body instruction
; %4 - repeat count, defaults to 100 - values other than 100 mean the Mops value will be wrong
%macro test_func 3-4 100
define_func %1
%2
.top:
times %4 %3
sub rdi, 100
jnz .top
ret
%endmacro

; pause
test_func pause_only,     {},             {pause}, 1

; vpermw latency
test_func avx512_vpermw,  {vpcmpeqd ymm0, ymm0, ymm0}, {vpermw  zmm0, zmm0, zmm0}

; vpermb latency
test_func avx512_vpermd,  {vpcmpeqd ymm0, ymm0, ymm0}, {vpermd  zmm0, zmm0, zmm0}

; imul latency
test_func avx128_imul,    {vpcmpeqd xmm0, xmm0, xmm0}, {vpmuldq xmm0, xmm0, xmm0}
test_func avx256_imul,    {vpcmpeqd ymm0, ymm0, ymm0}, {vpmuldq ymm0, ymm0, ymm0}
test_func avx512_imul,    {vpcmpeqd ymm0, ymm0, ymm0}, {vpmuldq zmm0, zmm0, zmm0}

; imul throughput
test_func avx128_imul_t,  {vpcmpeqd xmm0, xmm0, xmm0}, {vpmuldq xmm0, xmm1, xmm1}
test_func avx256_imul_t,  {vpcmpeqd ymm0, ymm0, ymm0}, {vpmuldq ymm0, ymm1, ymm1}
test_func avx512_imul_t,  {vpcmpeqd ymm0, ymm0, ymm0}, {vpmuldq zmm0, zmm1, zmm1}

; iadd latency
test_func scalar_iadd,    {xor eax, eax}, {add rax, rax}

test_func avx128_iadd,    {vpcmpeqd xmm0, xmm0, xmm0}, {vpaddq  xmm0, xmm0, xmm0}
test_func avx256_iadd,    {vpcmpeqd ymm0, ymm0, ymm0}, {vpaddq  ymm0, ymm0, ymm0}
test_func avx512_iadd,    {vpcmpeqd ymm0, ymm0, ymm0}, {vpaddq  zmm0, zmm0, zmm0}

; iadd latency with zmm16
test_func avx128_iadd16,  {vpternlogd xmm16, xmm16, xmm16, 0xff}, {vpaddq  xmm16, xmm16, xmm16}
test_func avx256_iadd16,  {vpternlogd ymm16, ymm16, ymm16, 0xff}, {vpaddq  ymm16, ymm16, ymm16}
test_func avx512_iadd16,  {vpternlogd zmm16, zmm16, zmm16, 0xff}, {vpaddq  zmm16, zmm16, zmm16}

; iadd throughput
test_func avx128_iadd_t,  {vpcmpeqd xmm1, xmm0, xmm0}, {vpaddq  xmm0, xmm1, xmm1}
test_func avx256_iadd_t,  {vpcmpeqd ymm1, ymm0, ymm0}, {vpaddq  ymm0, ymm1, ymm1}

; zeroing xor
test_func avx128_xor_zero, {}, {vpxor  xmm0, xmm0, xmm0}
test_func avx256_xor_zero, {}, {vpxor  ymm0, ymm0, ymm0}
test_func avx512_xor_zero, {}, {vpxord zmm0, zmm0, zmm0}

; vpsrlvd latency
test_func avx128_vshift,  {vpcmpeqd xmm1, xmm0, xmm0}, {vpsrlvd  xmm0, xmm0, xmm0}
test_func avx256_vshift,  {vpcmpeqd xmm1, xmm0, xmm0}, {vpsrlvd  ymm0, ymm0, ymm0}
test_func avx512_vshift,  {vpcmpeqd xmm1, xmm0, xmm0}, {vpsrlvd  zmm0, zmm0, zmm0}

; vpsrlvd throughput
test_func avx128_vshift_t,{vpcmpeqd xmm1, xmm0, xmm0}, {vpsrlvd  xmm0, xmm1, xmm1}
test_func avx256_vshift_t,{vpcmpeqd xmm1, xmm0, xmm0}, {vpsrlvd  ymm0, ymm1, ymm1}
test_func avx512_vshift_t,{vpcmpeqd xmm1, xmm0, xmm0}, {vpsrlvd  zmm0, zmm1, zmm1}

; vplzcntd latency
test_func avx128_vlzcnt,  {vpcmpeqd xmm1, xmm0, xmm0}, {vplzcntd  xmm0, xmm0}
test_func avx256_vlzcnt,  {vpcmpeqd xmm1, xmm0, xmm0}, {vplzcntd  ymm0, ymm0}
test_func avx512_vlzcnt,  {vpcmpeqd xmm1, xmm0, xmm0}, {vplzcntd  zmm0, zmm0}

; vplzcntd throughput
test_func avx128_vlzcnt_t,{vpcmpeqd xmm1, xmm0, xmm0}, {vplzcntd  xmm0, xmm1}
test_func avx256_vlzcnt_t,{vpcmpeqd xmm1, xmm0, xmm0}, {vplzcntd  ymm0, ymm1}
test_func avx512_vlzcnt_t,{vpcmpeqd xmm1, xmm0, xmm0}, {vplzcntd  zmm0, zmm1}

; FMA
test_func avx128_fma ,    {vpxor    xmm0, xmm0, xmm0}, {vfmadd132pd xmm0, xmm0, xmm0}
test_func avx256_fma ,    {vpxor    xmm0, xmm0, xmm0}, {vfmadd132pd ymm0, ymm0, ymm0}
test_func avx512_fma ,    {vpxor    xmm0, xmm0, xmm0}, {vfmadd132pd zmm0, zmm0, zmm0}

; this is like test_func, but it uses 10 parallel chains of instructions,
; unrolled 10 times, so (probably) max throughput at least if latency * throughput
; product for the instruction <= 10
; %1 - function name
; %2 - init instruction (e.g., xor out the variable you'll add to)
; %3 - register base like xmm, ymm, zmm
; %4 - loop body instruction only (no operands)
; %5 - init value for xmm0-9, used as first (dest) arg as in vfmadd132pd xmm0..9, xmm10, xmm11
; %6 - init value for xmm10, used as second arg as in vfmadd132pd reg, xmm10, xmm11
; %7 - init value for xmm11, used as third  arg as in vfmadd132pd reg, xmm10, xmm11
%macro test_func_tput 7
define_func %1

; init reg 0-9
%assign r 0
%rep 10
%2 %3 %+ r, %5
%assign r (r+1)
%endrep

; init reg10, reg11
%2 %3 %+ 10, %6
%2 %3 %+ 11, %7

.top:
%rep 10
%assign r 0
%rep 10
%4 %3 %+ r, %3 %+ 10, %3 %+ 11
%assign r (r+1)
%endrep
%endrep
sub rdi, 100
jnz .top
ret
%endmacro

test_func_tput avx128_fma_t ,   vmovddup,     xmm, vfmadd132pd, [zero_dp], [one_dp], [half_dp]
test_func_tput avx256_fma_t ,   vbroadcastsd, ymm, vfmadd132pd, [zero_dp], [one_dp], [half_dp]
test_func_tput avx512_fma_t ,   vbroadcastsd, zmm, vfmadd132pd, [zero_dp], [one_dp], [half_dp]
test_func_tput avx512_vpermw_t ,vbroadcastsd, zmm, vpermw,      [zero_dp], [one_dp], [half_dp]
test_func_tput avx512_vpermd_t ,vbroadcastsd, zmm, vpermd,      [zero_dp], [one_dp], [half_dp]

; this is like test_func except that the 100x unrolled loop instruction is
; always a serial scalar add, while the passed instruction to test is only
; executed once per loop (so at a ratio of 1:100 for the scalar adds). This
; test the effect of an "occasional" AVX instruction.
; %1 - function name
; %2 - init instruction (e.g., xor out the variable you'll add to)
; %3 - loop body instruction
%macro test_func_sparse 4
define_func %1
%2
%4
xor eax, eax
.top:
%3
times 100 add eax, eax
sub rdi, 100
jnz .top
ret
%endmacro

test_func_sparse avx128_mov_sparse,       {vbroadcastsd ymm0, [one_dp]}, {vmovdqa xmm0, xmm0}, {}
test_func_sparse avx256_mov_sparse,       {vbroadcastsd ymm0, [one_dp]}, {vmovdqa ymm0, ymm0}, {}
test_func_sparse avx512_mov_sparse,       {vbroadcastsd zmm0, [one_dp]}, {vmovdqa32 zmm0, zmm0}, {}
test_func_sparse avx128_merge_sparse, {vbroadcastsd ymm0, [one_dp]}, {vmovdqa32 xmm0{k1}, xmm0}, {kmovw k1, [kmask]}
test_func_sparse avx256_merge_sparse, {vbroadcastsd ymm0, [one_dp]}, {vmovdqa32 ymm0{k1}, ymm0}, {kmovw k1, [kmask]}
test_func_sparse avx512_merge_sparse, {vbroadcastsd zmm0, [one_dp]}, {vmovdqa32 zmm0{k1}, zmm0}, {kmovw k1, [kmask]}

test_func_sparse avx128_fma_sparse, {vbroadcastsd ymm0, [zero_dp]}, {vfmadd132pd xmm0, xmm0, xmm0 }, {}
test_func_sparse avx256_fma_sparse, {vbroadcastsd ymm0, [zero_dp]}, {vfmadd132pd ymm0, ymm0, ymm0 }, {}
test_func_sparse avx512_fma_sparse, {vbroadcastsd zmm0, [zero_dp]}, {vfmadd132pd zmm0, zmm0, zmm0 }, {}

; %1 function name suffix
; %2 dirty instruction
%macro define_ucomis 2
define_func ucomis_%1
;vpxor xmm15, xmm15, xmm15
;vzeroupper
%2
movdqu xmm0, [one_dp]
movdqu xmm2, [one_dp]
movdqu xmm1, [zero_dp]
align 64
.top:
%rep 100
addsd   xmm0, xmm2
ucomisd xmm1, xmm0
ja .never
%endrep
sub rdi, 100
jnz .top
ret
.never:
ud2
%endmacro

define_ucomis clean, {vzeroupper}
define_ucomis dirty, {}


define_func dirty_it
vzeroupper
vpxord zmm15, zmm14, zmm15
ret

define_func dirty_it16
vzeroupper
vpxord zmm16, zmm14, zmm15
ret

GLOBAL zeroupper_asm:function
zeroupper_asm:
vzeroupper
ret

zero_dp: dq 0.0
half_dp: dq 0.5
one_dp:  dq 1.0
kmask:   dq 0x5555555555555555



