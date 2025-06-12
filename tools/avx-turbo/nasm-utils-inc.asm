;; potentially useful macros for asm development

;; long-nop instructions: nopX inserts a nop of X bytes
;; see "Table 4-12. Recommended Multi-Byte Sequence of NOP Instruction" in
;; "Intel® 64 and IA-32 Architectures Software Developer’s Manual" (325383-061US)
%define nop1 nop                                                     ; just a nop, included for completeness
%define nop2 db 0x66, 0x90                                           ; 66 NOP
%define nop3 db 0x0F, 0x1F, 0x00                                     ;    NOP DWORD ptr [EAX]
%define nop4 db 0x0F, 0x1F, 0x40, 0x00                               ;    NOP DWORD ptr [EAX + 00H]
%define nop5 db 0x0F, 0x1F, 0x44, 0x00, 0x00                         ;    NOP DWORD ptr [EAX + EAX*1 + 00H]
%define nop6 db 0x66, 0x0F, 0x1F, 0x44, 0x00, 0x00                   ; 66 NOP DWORD ptr [EAX + EAX*1 + 00H]
%define nop7 db 0x0F, 0x1F, 0x80, 0x00, 0x00, 0x00, 0x00             ;    NOP DWORD ptr [EAX + 00000000H]
%define nop8 db 0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00       ;    NOP DWORD ptr [EAX + EAX*1 + 00000000H]
%define nop9 db 0x66, 0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00 ; 66 NOP DWORD ptr [EAX + EAX*1 + 00000000H]

;; push the 6 callee-saved registers defined in the the SysV C ABI
%macro push_callee_saved 0
push rbp
push rbx
push r12
push r13
push r14
push r15
%endmacro

;; pop the 6 callee-saved registers in the order compatible with push_callee_saved
%macro pop_callee_saved 0
pop r15
pop r14
pop r13
pop r12
pop rbx
pop rbp
%endmacro

EXTERN nasm_util_assert_failed

; place the string value of a tken in .rodata using %defstr
; arg1 - the token to make into a string
; arg2 - label which will point to the string
%macro make_string_tok 2
%ifdef __YASM_MAJOR__
; yasm has no support for defstr so we just use a fixed string for now
; see https://github.com/travisdowns/nasm-utils/issues/1
make_string 'make_string_tok yasm bug', %2
%else
%defstr make_string_temp %1
make_string make_string_temp, %2
%endif
%endmacro

%macro make_string 2
[section .rodata]
%2:
db %1,0
; restore the previous section
__SECT__
%endmacro

%macro nasm_util_assert_boilerplate 0
make_string __FILE__, parent_filename
%define ASSERT_BOILERPLATE 1
%endmacro

%macro check_assert_boilerplate 0
%ifndef ASSERT_BOILERPLATE
%error "To use asserts, you must include a call to nasm_util_assert_boilerplate once in each file"
%endif
%endmacro

;; assembly level asserts
;; if the assert occurs, termination is assumed so control never passes back to the caller
;; and registers are not preserved
%macro assert_eq 2
check_assert_boilerplate
cmp     %1, %2
je      %%assert_ok
make_string_tok %1, %%assert_string1
make_string_tok %2, %%assert_string2
lea     rdi, [%%assert_string1]
lea     rsi, [%%assert_string2]
lea     rdx, [parent_filename]
mov     rcx, __LINE__
jmp     nasm_util_assert_failed
%%assert_ok:
%endmacro

;; boilerplate needed once when abi_checked_function is used
%macro thunk_boilerplate 0
; this function is defined by the C helper code
EXTERN nasm_util_die_on_reg_clobber

boil1 rbp, 1
boil1 rbx, 2
boil1 r12, 3
boil1 r13, 4
boil1 r14, 5
boil1 r15, 6
%endmacro

;; By default, the "assert-like" features that can be conditionally enabled key off the value of the
;; NDEBUG macro: if it is defined, the slower, more heavily checked paths are enabled, otherwise they
;; are omitted (usually resulting in zero additional cost).
;;
;; If you don't want to rely on NDEBUG can specifically enable or disable the debug mode with the
;; NASM_ENABLE_DEBUG set to 0 (equivalent to NDEBUG set) or 1 (equivalent to NDEBUG not set)
%ifndef NASM_ENABLE_DEBUG
    %ifdef NDEBUG
        %define NASM_ENABLE_DEBUG 0
    %else
        %define NASM_ENABLE_DEBUG 1
    %endif
%elif (NASM_ENABLE_DEBUG != 0) && (NASM_ENABLE_DEBUG != 1)
    %error bad value for 'NASM_ENABLE_DEBUG': should be 0 or 1 but was NASM_ENABLE_DEBUG
%endif




;; This macro supports declaring a "ABI-checked" function in asm
;; An ABI-checked function will checked at each invocation for compliance with the SysV ABI
;; rules about callee saved registers. In particular, from the ABI cocument we have the following:
;;
;;      Registers %rbp, %rbx and %r12 through %r15 “belong” to the calling function
;;      and the called function is required to preserve their values.
;;            (from "System V Application Binary Interface, AMD64 Architecture Processor Supplement")
;;
;;
%macro abi_checked_function 1
GLOBAL %1:function

%1:

%if NASM_ENABLE_DEBUG != 0

;%warning compiling ABI checks

; save all the callee-saved regs
push_callee_saved
push    rax  ; dummy push to align the stack (before we have rsp % 16 == 8)
call %1_inner
add     rsp, 8 ; undo dummy push

; load the function name (ok to clobber rdi since it's callee-saved)
mov rdi, %1_thunk_fn_name

; now check whether any regs were clobbered
cmp rbp, [rsp + 40]
jne bad_rbp
cmp rbx, [rsp + 32]
jne bad_rbx
cmp r12, [rsp + 24]
jne bad_r12
cmp r13, [rsp + 16]
jne bad_r13
cmp r14, [rsp + 8]
jne bad_r14
cmp r15, [rsp]
jne bad_r15

add rsp, 6 * 8
ret


; here we store strings needed by the failure cases, in the .rodata section
[section .rodata]
%1_thunk_fn_name:
%ifdef __YASM_MAJOR__
; yasm doesn't support defstr, so for now just use an unknown name
db "unknown (see yasm issue #95)",0
%else
%defstr fname %1
db fname,0
%endif

; restore the previous section
__SECT__

%1_inner:
%endif ; debug off, just assemble the function as-is without any checks

%endmacro


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; IMPLEMENTATION FOLLOWS
;; below you find internal macros needed for the implementation of the above macros
;;;;;;;;;;;;;;;;;;;;;;;;;;;

; generate the stubs for the bad_reg functions called from the check-abi thunk
%macro boil1 2
bad_%1:
; A thunk has determined that a reg was clobbered
; each reg has their own bad_ function which moves the function name (in rdx) into
; rdi and loads a constant indicating which reg was involved and calls a C routine
; that will do the rest (abort the program generall). We follow up with an ud2 in case
; the C routine returns, since this mechanism is not designed for recovery.
mov rsi, %2
; here we set up a stack frame - this gives a meaningful backtrace in any core file produced by the abort
; first we need to pop the saved regs off the stack so the rbp chain is consistent
add rsp, 6 * 8
push rbp
mov  rbp, rsp
call nasm_util_die_on_reg_clobber
ud2
%endmacro




