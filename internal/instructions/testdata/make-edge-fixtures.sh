#!/usr/bin/env bash
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# Generate synthetic perf-annotate fixtures that exercise classifier branches the real
# workload never reaches, for differential testing of the register-pressure analyzer.
set -euo pipefail

OUT="${1:?usage: make-edge-fixtures.sh <outdir>}"
mkdir -p "$OUT"

hdr() {
	printf ' Percent |\tSource code & Disassembly of x for cycles:upp (100 samples, percent: local period)\n'
	printf -- '---------------------------------------------------------------\n'
	printf '         :\n'
	printf '         : 5    0000000000001000 <%s>:\n' "$1"
}

# emit <pct> <addr> <insn...>
emit() { printf '   %5s :   %s:   %s\n' "$1" "$2" "$3"; }

# A: rsp-based spills, no frame-pointer prologue. rbp must be treated as a plain GP reg.
{
	hdr nofp
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'sub    $0x28,%rsp'
	emit 20.00 1008 'mov    %rax,0x8(%rsp)'
	emit 20.00 100d 'mov    0x8(%rsp),%rbx'
	emit 20.00 1012 'mov    %rbp,0x10(%rsp)'
	emit 20.00 1017 'mov    -0x8(%rbp),%rcx'
	emit 20.00 101b 'add    $0x28,%rsp'
	emit 0.00 101f 'ret'
} >"$OUT/edge_nofp.stdio"

# B: stack canary. The %fs load, its store, and the exit compare must all be excluded.
{
	hdr canary
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 10.00 1008 'mov    %fs:0x28,%rax'
	emit 10.00 1011 'mov    %rax,-0x8(%rsp)'
	emit 40.00 1015 'mov    %r12,-0x20(%rbp)'
	emit 40.00 101a 'mov    -0x28(%rbp),%r13'
	emit 10.00 101f 'sub    -0x8(%rsp),%rax'
	emit 0.00 1024 'ret'
} >"$OUT/edge_canary.stdio"

# C: push/pop as stack traffic, and lea on a stack address which must NOT count.
{
	hdr pushpop
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 25.00 1008 'push   %r15'
	emit 25.00 100a 'pop    %r14'
	emit 25.00 100c 'lea    -0x10(%rbp),%rsi'
	emit 25.00 1010 'lea    0x8(%rsp),%rdi'
	emit 0.00 1015 'ret'
} >"$OUT/edge_pushpop_lea.stdio"

# D: read-modify-write on a stack slot counts as both a spill and a reload, and cmp/test
# against a stack slot counts as a load only.
{
	hdr rmw
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 25.00 1008 'add    %rax,-0x18(%rbp)'
	emit 25.00 100c 'xor    %r15,-0x50(%rbp)'
	emit 25.00 1010 'cmpl   $0x0,-0x8(%rbp)'
	emit 25.00 1014 'testb  $0x1,-0x9(%rbp)'
	emit 0.00 1018 'ret'
} >"$OUT/edge_rmw_cmp.stdio"

# E: SIMD spill/reload across widths, including a vector op with a GP index register.
{
	hdr simd
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 20.00 1008 'movaps %xmm3,-0x30(%rbp)'
	emit 20.00 100c 'movups -0x40(%rbp),%xmm7'
	emit 20.00 1010 'vmovdqa %ymm2,-0x80(%rbp)'
	emit 20.00 1018 'vmovdqu64 -0x100(%rbp),%zmm9'
	emit 20.00 1020 'movsd  %xmm15,-0x18(%rbp,%rax,8)'
	emit 0.00 1028 'ret'
} >"$OUT/edge_simd.stdio"

# F: sub-register widths, to exercise register-name normalization in the live census.
{
	hdr widths
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 14.00 1008 'mov    %eax,-0x4(%rbp)'
	emit 14.00 100b 'mov    %r9d,-0x8(%rbp)'
	emit 14.00 100f 'mov    %bx,-0xa(%rbp)'
	emit 14.00 1013 'mov    %r10w,-0xc(%rbp)'
	emit 14.00 1018 'mov    %cl,-0xd(%rbp)'
	emit 15.00 101b 'mov    %sil,-0xe(%rbp)'
	emit 15.00 101f 'mov    %r11b,-0xf(%rbp)'
	emit 0.00 1023 'ret'
} >"$OUT/edge_widths.stdio"

# G: single-operand and immediate-source stack writes; the immediate store has no source
# register, so it is a spill with neither GP nor SIMD attribution.
{
	hdr imm
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 25.00 1008 'movq   $0x0,-0x20(%rbp)'
	emit 25.00 1010 'incl   -0x8(%rbp)'
	emit 25.00 1013 'negq   -0x10(%rbp)'
	emit 25.00 1017 'notq   -0x18(%rbp)'
	emit 0.00 101b 'ret'
} >"$OUT/edge_imm.stdio"

# H: no samples anywhere. Every weight is zero, so the denominator guard is exercised.
{
	hdr zero
	emit 0.00 1000 'endbr64'
	emit 0.00 1004 'push   %rbp'
	emit 0.00 1005 'mov    %rsp,%rbp'
	emit 0.00 1008 'mov    %rax,-0x8(%rbp)'
	emit 0.00 100c 'ret'
} >"$OUT/edge_zero.stdio"

ls -1 "$OUT"/edge_*.stdio
