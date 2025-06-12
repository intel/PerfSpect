/*
 * tsc-support.cpp
 */

#include <cinttypes>
#include <string>

#ifdef _MSC_VER
#include <intrin.h>
#else
#include <x86intrin.h>
#endif

static inline uint64_t rdtsc() {
    return __rdtsc();
}

/**
 * Get the TSC frequency.
 *
 * By default, this tries to read the TSC frequency directly from cpuid leaf 0x15,
 * if it is on a supported architecture, otherwise it falls back to using a calibration
 * loop. If force_calibrate is true, it always uses the calibration loop and never reads
 * from cpuid.
 */
std::uint64_t get_tsc_freq(bool force_calibrate);

/** return a string describing how the TSC frequency was determined */
const char* get_tsc_cal_info(bool force_calibrate);
