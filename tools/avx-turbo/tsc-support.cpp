/*
 * tsc-support.cpp
 */

#include "tsc-support.hpp"
#include "cpuid.hpp"

#include <cinttypes>
#include <string>
#include <cstdio>
#include <cassert>
#include <array>
#include <algorithm>
#include <numeric>

#include <error.h>
#include <time.h>

using std::uint32_t;



uint64_t get_tsc_from_cpuid_inner() {
    if (cpuid_highest_leaf() < 0x15) {
        std::printf("CPUID doesn't support leaf 0x15, falling back to manual TSC calibration.\n");
        return 0;
    }

    auto cpuid15 = cpuid(0x15);
    std::printf("cpuid = %s\n", cpuid15.to_string().c_str());

    if (cpuid15.ecx) {
        // the crystal frequency was present in ECX
        return (uint64_t)cpuid15.ecx * cpuid15.ebx / cpuid15.eax;
    }

    // ecx == 0 means we have to use a hard-coded frequency based on the model and table provided by Intel
    // in 18.7.3
    auto family = get_family_model();
    std::printf("cpu: %s\n", family.to_string().c_str());


    if (family.family == 6) {
        if (family.model == 0x4E || family.model == 0x5E || family.model == 0x8E || family.model == 0x9E) {
            // skylake client or kabylake
            return (int64_t)24000000 * cpuid15.ebx / cpuid15.eax; // 24 MHz crystal clock
        }
    } else {
        std::printf("CPU family not 6 (perhaps AMD or old Intel), falling back to manual TSC calibration.\n");
    }

    return 0;
}

uint64_t get_tsc_from_cpuid() {
    static auto cached = get_tsc_from_cpuid_inner();
    return cached;
}


namespace Clock {
    static inline uint64_t nanos() {
        struct timespec ts;
        clock_gettime(CLOCK_MONOTONIC, &ts);
        return (uint64_t)ts.tv_sec * 1000000000 + ts.tv_nsec;
    }
}

constexpr size_t SAMPLES = 101;
constexpr uint64_t DELAY_NANOS = 10000; // nanos 1us

uint64_t do_sample() {
    _mm_lfence();
    uint64_t  nsbefore = Clock::nanos();
    uint64_t tscbefore = rdtsc();
    while (nsbefore + DELAY_NANOS > Clock::nanos())
        ;
    uint64_t  nsafter = Clock::nanos();
    uint64_t tscafter = rdtsc();
    return (tscafter - tscbefore) * 1000000000u / (nsafter - nsbefore);
}

uint64_t tsc_from_cal() {
    std::array<uint64_t, SAMPLES * 2> samples;

    for (size_t s = 0; s < SAMPLES * 2; s++) {
        samples[s] = do_sample();
    }

    // throw out the first half of samples as a warmup
    std::array<uint64_t, SAMPLES> second_half;
    std::copy(samples.begin() + SAMPLES, samples.end(), second_half.begin());
    std::sort(second_half.begin(), second_half.end());

    // average the middle quintile
    auto third_quintile = second_half.begin() + 2 * SAMPLES/5;
    uint64_t sum = std::accumulate(third_quintile, third_quintile + SAMPLES/5, (uint64_t)0);

    return sum / (SAMPLES/5);
}

/**
 * TSC frequency detection is described in
 * Intel SDM Vol3 18.7.3: Determining the Processor Base Frequency
 *
 * Nominal TSC frequency = ( CPUID.15H.ECX[31:0] * CPUID.15H.EBX[31:0] ) ÷ CPUID.15H.EAX[31:0]
 */
uint64_t get_tsc_freq(bool force_calibrate) {
    uint64_t tsc;
    if (!force_calibrate && (tsc = get_tsc_from_cpuid())) {
        return tsc;
    }

    return tsc_from_cal();
}


const char* get_tsc_cal_info(bool force_calibrate) {
    if (!force_calibrate && get_tsc_from_cpuid()) {
        return "from cpuid leaf 0x15";
    } else {
        return "from calibration loop";
    }

}







