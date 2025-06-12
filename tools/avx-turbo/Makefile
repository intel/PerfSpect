include config.mk

# rebuild when makefile changes
-include dummy.rebuild

.PHONY: all clean

ASM_FLAGS ?= -DNASM_ENABLE_DEBUG=$(NASM_DEBUG) -w+all -l x86_methods.list

ifneq ($(CPU_ARCH),)
ARCH_FLAGS := -march=$(CPU_ARCH)
endif
O_LEVEL ?= -O2

COMMON_FLAGS := -MMD -Wall -Wextra -Wundef $(ARCH_FLAGS) -g $(O_LEVEL)
CPPFLAGS := $(COMMON_FLAGS)
CFLAGS := $(COMMON_FLAGS)

SRC_FILES := $(wildcard *.cpp) $(wildcard *.c)

OBJECTS := $(SRC_FILES:.cpp=.o) asm-methods.o
OBJECTS := $(OBJECTS:.c=.o)
DEPFILES = $(OBJECTS:.o=.d)
# $(info OBJECTS=$(OBJECTS))

VPATH = test

###########
# Targets #
###########

all: avx-turbo unit-test

-include $(DEPFILES) unit-test.d

clean:
	rm -f *.d *.o avx-turbo

dist-clean: clean $(CLEAN_TARGETS)

unit-test: unit-test.o unit-test-main.o cpuid.o
	$(CXX) $(CPPFLAGS) $(CXXFLAGS) $(LDFLAGS) $(LDLIBS) -std=c++11 $^ -o $@

avx-turbo: $(OBJECTS)
	$(CXX) $(OBJECTS) $(CPPFLAGS) $(CXXFLAGS) $(LDFLAGS) $(LDLIBS) -std=c++11 -lpthread -o $@

%.o : %.c
	$(CC) $(CFLAGS) -c -std=c11 -o $@ $<

%.o : %.cpp
	$(CXX) $(CPPFLAGS) $(CXXFLAGS) -c -std=c++11 -o $@ $<

%.o: %.asm nasm-utils-inc.asm
	$(ASM) $(ASM_FLAGS) -f elf64 $<

LOCAL_MK = $(wildcard local.mk)

# https://stackoverflow.com/a/3892826/149138
dummy.rebuild: Makefile config.mk $(LOCAL_MK)
	touch $@
	$(MAKE) -s clean
