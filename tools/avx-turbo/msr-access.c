/*
 * msr-access.c
 */

// for pread() and sched_getcpu()
#define _GNU_SOURCE

#include "msr-access.h"

#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <errno.h>
#include <inttypes.h>
#include <assert.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>
#include <sched.h>

/** if there are this many CPUs or less, we'll never allocate memory */
#define STATIC_ARRAY_SIZE 32

#ifndef MSR_USE_PTHREADS
// thread-safe by default
#define MSR_USE_PTHREADS 1
#endif

#if MSR_USE_PTHREADS
#include <pthread.h>
static pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;
void lock() {
    pthread_mutex_lock(&mutex);
}
void unlock() {
    pthread_mutex_unlock(&mutex);
}
#else
void lock() {}
void unlock(){}
#endif



/* size of the rfile array */
int  rfile_static[STATIC_ARRAY_SIZE] = {};
int  rfile_size  = STATIC_ARRAY_SIZE;
int *rfile_array = rfile_static;
//int rfile_error;

/** get the read-only file associated with the given cpu */
int get_rfile(int cpu) {
    assert(cpu >= 0);

    lock();

    if (cpu >= rfile_size) {
        // expand array
        size_t new_size = rfile_size * 2 > cpu ? rfile_size * 2 : cpu;
        int *new_array = calloc(new_size, sizeof(int));
        memcpy(new_array, rfile_array, rfile_size  * sizeof(int));
        if (rfile_array != rfile_static) {
            free(rfile_array);
        }
        rfile_array = new_array;
        rfile_size  = new_size;
    }

    if (rfile_array[cpu] == 0) {
        char filename[64] = {};
        int ret = snprintf(filename, 64, "/dev/cpu/%d/msr", cpu);
        assert(ret > 0);
        rfile_array[cpu] = open(filename, O_RDONLY);
        if (rfile_array[cpu] == -1) {
            rfile_array[cpu] = -errno;
        }
    }

    int ret = rfile_array[cpu];

    unlock();

    return ret;
}

int read_msr(int cpu, uint32_t msr_index, uint64_t* value) {
    int file = get_rfile(cpu);
    assert(file);
    if (file < 0) {
        // file open failes are stored as negative errno
        return file;
    }
    int read = pread(file, value, 8, msr_index);
    return read == -1 ? errno : 0;
}

int read_msr_cur_cpu(uint32_t msr_index, uint64_t* value) {
    return read_msr(sched_getcpu(), msr_index, value);
}


// rename this to main to build an exe that can be run as ./a.out CPU MSR
// to read MSR from CPU (like a really simple rdmsr)
int test(int argc, char** argv) {
    assert(argc == 3);
    int cpu      = atoi(argv[1]);
    uint32_t msr = atoi(argv[2]);
    printf("reading msr %u from cpu %d\n", msr, cpu);
    uint64_t value = -1;

    int res = read_msr(cpu, msr, &value);
    if (res) {
        printf("error %d\n", res);
    } else {
        printf("value %lx\n", value);
    }

    res = read_msr_cur_cpu(msr, &value);
    if (res) {
        printf("error %d\n", res);
    } else {
        printf("value %lx\n", value);
    }

    return EXIT_SUCCESS;
}


