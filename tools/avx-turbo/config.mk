-include local.mk

# set DEBUG to 1 to enable various debugging checks
DEBUG ?= 0

# The assembler to use. Defaults to nasm, but can also be set to yasm which has better
# debug info handling.
ASM ?= ./nasm-2.13.03/nasm

ifeq ($(DEBUG),1)
O_LEVEL ?= -O0
NASM_DEBUG ?= 1
else
O_LEVEL ?= -O2
NASM_DEBUG ?= 0
endif
