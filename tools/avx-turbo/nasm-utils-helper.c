/*
 * nasm-utils-helper.c
 *
 * C helper functions for some macros in nasm-utils-inc.asm.
 *
 * If you use any macros that require functionality defined here, just include this C file in
 * your project (linked against the same object that contains the assembly generated with the
 * help of nasm-utils-inc.asm).
 */

#include <stdio.h>
#include <stdlib.h>
#include <inttypes.h>
#include <assert.h>


// mapping from reg_id to register name
static const char *reg_names[] = {
        "rbp",
        "rbx",
        "r12",
        "r13",
        "r14",
        "r15"
};

/* called when a function using abi_checked_function detects an illegally clobbered register */
void nasm_util_die_on_reg_clobber(const char *fname, unsigned reg_id) {
    reg_id--; // reg ids are 1-based
    if (reg_id >= sizeof(sizeof(reg_names)/sizeof(reg_names[0]))) {
        fprintf(stderr, "FATAL: function %s clobbered a callee-saved register (thunk returned an invalid reg_id %d)\n",
                fname, reg_id);
    } else {
        fprintf(stderr, "FATAL: function %s clobbered callee-saved register %s\n", fname, reg_names[reg_id]);
    }
    abort();
}

void nasm_util_assert_failed(const char *left, const char *right, const char *filename, int64_t line) {
    fprintf(stderr, "%s:%ld : Assertion failed: %s == %s\n", filename, (long)line, left, right);
    fflush(stderr);
    abort();
}
