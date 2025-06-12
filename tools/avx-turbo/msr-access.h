/*
 * msr-access.h
 *
 * Simple API to access the x86 MSR registers exposed on linux with through the /dev/cpu/N/msr file system.
 *
 * Unless you've changed the msr permissions, only root can do this. The msr filesystem may not exist until
 * 'modprobe msr' is executed to load the msr module.
 */

#ifndef MSR_ACCESS_H_
#define MSR_ACCESS_H_

#include <inttypes.h>
// you could get the MSR index values from the following header, although it isn't exported to user-space
// in kernels after 4.12, but you can grab it from the linux source
// #include <asm/msr-index.h>

#ifdef __cplusplus
extern "C" {
#endif


/**
 * Read the MSR given by msr_index on the given cpu, storing the result into
 * result, which must point to at least 8 bytes of storage.
 *
 * Returns zero on success, non-zero on failure.
 *
 * Negative values indicate errors
 * opening the underlying MSR file: the value returned is the negative of the errno
 * returned by the kernel when trying to open the file. These file errors is cached
 * so once a negative value has been returned for a given cpu, subsequent calls will
 * always return the same value.
 *
 * Positive values indicate failures during the pread call performed to actually read
 * the msr from the open file. The value is the errno returned by the kernel after the
 * read. The most common value is 5 (EIO) which indicates that you can't read that MSR
 * on this hardware (e.g., if may not exist).
 */
int read_msr(int cpu, uint32_t msr_index, uint64_t* value);

/**
 * Reads the given MSR on the current CPU. This is just a shortcut for calling
 * read_msr(sched_getcpu(), ...), and the result and error handling is the same as that function.
 *
 * Of course, unless the thread affinity has been restricted for the current thread,
 * the result doesn't help the calling code know the true value on the current CPU since
 * a context switch can happen at any time (the same caveat applies to getcpu()).
 */
int read_msr_cur_cpu(uint32_t msr_index, uint64_t* value);


#ifdef __cplusplus
} // extern "C" {
#endif

#endif // #ifdef MSR_ACCESS_H_
