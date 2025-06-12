# avx-turbo

Test the non-AVX, AVX2 and AVX-512 speeds for various types of CPU intensive loops with varying scalar and SIMD instructions, across different active core counts.

Currently it is **Linux only** (it does run on WSL and WSL2 on Windows), but the basic testing mechanism could be ported to OSX and Windows as well (help welcome). 

# CI Status

**Build:** [![Master Branch](https://github.com/travisdowns/avx-turbo/workflows/build/badge.svg)](https://github.com/travisdowns/avx-turbo/actions?query=workflow%3Abuild+branch%3Amaster)


# build

    make
    
# msr kernel module

You should load the `msr` kernel module if it is not already loaded. This is as simple as:

    modprobe msr

Or as complex as (if you want nice messages about what happened):

    lsmod | grep -q msr && echo "MSR already loaded" || { echo "Loading MSR module"; sudo modprobe msr ; }

# run

You get the most info running as root (since we can read various MSRs to calculate the frequency directly):

    sudo ./avx-turbo

You can also run it without root, but you only get the "Mops" reading (but this can be read directly as frequency
for the 1-latency tests). 

## spec-based tests

The default behavior for ./avx-turbo is to run tests with various thread counts, but with the same test on each thread. For example, the `avx256_fma` test means that the same FMA-using test code will be run on _each_ test thread.

An alternate approach is availe with so-called _spec-based_ tests. This lets you specificy exactly what each thread in a test will run. The general form of a specification is: `test1/thead-count1[,test2/thread-count2[,...]]`. For example,
if you run `sudo ./avx-turbo --spec avx256_fma/1,scalar_iadd/3` you'll get one copy of `avx256_fma` and three copies of `scalar_iadd` running in parallel.

This mode is useful to testing that happens when not all cores are doing the same thing.

# help

Try:

    ./avx-turbo --help
    
for a summary of some options something like this:

```
  ./avx-turbo {OPTIONS}

    avx-turbo: Determine AVX2 and AVX-512 downclocking behavior

  OPTIONS:

      -h, --help                        Display this help menu
      --force-tsc-calibrate             Force manual TSC calibration loop, even
                                        if cpuid TSC Hz is available
      --no-pin                          Don't try to pin threads to CPU - gives
                                        worse results but works around affinity
                                        issues on TravisCI
      --verbose                         Output more info
      --no-barrier                      Don't sync up threads before each test
                                        (no real purpose)
      --list                            List the available tests and their
                                        descriptions
      --allow-hyperthreads              By default we try to filter down the
                                        available cpus to include only physical
                                        cores, but with this option we'll use
                                        all logical cores meaning you'll run two
                                        tests on cores with hyperthreading
      --test=[TEST-ID]                  Run only the specified test (by ID)
      --spec=[SPEC]                     Run a specific type of test specified by
                                        a specification string
      --iters=[ITERS]                   Run the test loop ITERS times (default
                                        100000)
      --min-threads=[MIN]               The minimum number of threads to use
      --max-threads=[MAX]               The maximum number of threads to use
      --warmup-ms=[MILLISECONDS]        Warmup milliseconds for each thread
                                        after pinning (default 100)

```

# output

The output looks like this:

```
Running as root     : [YES]
CPU supports AVX2   : [YES]
CPU supports AVX-512: [NO ]
cpuid = eax = 2, ebx = 216, ecx = 0, edx = 0
cpu: family = 6, model = 94, stepping = 3
tsc_freq = 2592.0 MHz (from cpuid leaf 0x15)
Will test up to 4 CPUs
============================== Threads:  1 ==============================
ID           | Description              | Mops | A/M-ratio | A/M-MHz | M/tsc-ratio
scalar_iadd  | Scalar integer adds      | 2594 |      1.00 |    2592 |        1.00
avx128_iadd  | 128-bit integer adds     | 2594 |      1.00 |    2592 |        1.00
avx128_imul  | 128-bit integer muls     |  519 |      1.00 |    2592 |        1.00
avx128_fma   | 128-bit 64-bit FMAs      |  649 |      1.00 |    2592 |        1.00
avx256_iadd  | 256-bit integer adds     | 2594 |      1.00 |    2592 |        1.00
avx256_imul  | 256-bit integer muls     |  519 |      1.00 |    2592 |        1.00
avx256_fma   | 256-bit serial DP FMAs   |  648 |      1.00 |    2592 |        1.00
avx256_fma_t | 256-bit parallel DP FMAs | 5189 |      1.00 |    2592 |        1.00
=========================================================================

============================== Threads:  2 ==============================
ID           | Description              |       Mops |    A/M-ratio |    A/M-MHz | M/tsc-ratio
scalar_iadd  | Scalar integer adds      | 2593, 2593 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx128_iadd  | 128-bit integer adds     | 2594, 2594 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx128_imul  | 128-bit integer muls     |  519,  519 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx128_fma   | 128-bit 64-bit FMAs      |  648,  649 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx256_iadd  | 256-bit integer adds     | 2594, 2594 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx256_imul  | 256-bit integer muls     |  519,  519 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx256_fma   | 256-bit serial DP FMAs   |  648,  648 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
avx256_fma_t | 256-bit parallel DP FMAs | 5188, 5189 |  1.00,  1.00 | 2592, 2592 |  1.00, 1.00
=========================================================================

============================== Threads:  3 ==============================
ID           | Description              |             Mops |           A/M-ratio |          A/M-MHz |      M/tsc-ratio
scalar_iadd  | Scalar integer adds      | 2594, 2594, 2594 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx128_iadd  | 128-bit integer adds     | 2594, 2594, 2594 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx128_imul  | 128-bit integer muls     |  519,  519,  519 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx128_fma   | 128-bit 64-bit FMAs      |  649,  648,  648 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx256_iadd  | 256-bit integer adds     | 2594, 2594, 2594 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx256_imul  | 256-bit integer muls     |  519,  519,  519 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx256_fma   | 256-bit serial DP FMAs   |  649,  648,  649 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
avx256_fma_t | 256-bit parallel DP FMAs | 5190, 5189, 5190 |  1.00,  1.00,  1.00 | 2592, 2592, 2592 | 1.00, 1.00, 1.00
=========================================================================

============================== Threads:  4 ==============================
ID           | Description              |                   Mops |                  A/M-ratio |                A/M-MHz |            M/tsc-ratio
scalar_iadd  | Scalar integer adds      | 2594, 2594, 2594, 2594 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx128_iadd  | 128-bit integer adds     | 2593, 2594, 2594, 2594 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx128_imul  | 128-bit integer muls     |  519,  519,  519,  519 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx128_fma   | 128-bit 64-bit FMAs      |  648,  648,  649,  648 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx256_iadd  | 256-bit integer adds     | 2594, 2594, 2594, 2594 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx256_imul  | 256-bit integer muls     |  519,  519,  519,  519 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx256_fma   | 256-bit serial DP FMAs   |  648,  648,  648,  648 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
avx256_fma_t | 256-bit parallel DP FMAs | 5189, 5189, 5189, 5189 |  1.00,  1.00,  1.00,  1.00 | 2592, 2592, 2592, 2592 | 1.00, 1.00, 1.00, 1.00
=========================================================================
```

The headings are:

 - `ID` The ID for the test, which you can use with the `--test` argument to only run a specific test (handy when you want to focus on one test to read the frequency externally, e.g., via `perf`).
 - `Description` Yes, it's a description.
 - `Mops` Million operations per second. Every test runs a loop of the same type of instruction and this is how many millions of those instructions were executed per second. This is handy since this value corresponds exactly to frequency in MHz for tests with serially dependent 1-latency instructions, which here are all the "integer adds" tests.
 - `A/M` This is the ratio of the `APERF` and `MPERF` ratios exposed in an MSR. For details, see the [Intel SDM Vol 3](https://software.intel.com/en-us/download/intel-64-and-ia-32-architectures-sdm-combined-volumes-3a-3b-3c-and-3d-system-programming-guide), but basically APERF is a free running counter of actual cycles (i.e., varying with the CPU frequency), while MPERF counts at a constant rate, usually the processor's nominal frequency. A ratio of 1.0 therefore means that the CPU was is running, on average, at the nominal frequency during the test (I had turbo off, that's why you see 1.00 everywhere). Lower than 1 means lower than nominal frequencies (e.g., due to running heavy AVX code).
 - `A/M-MHz` This is the measured frequency over the duration of the test, based on the `APERF` and `MPERF` ratio described above, multiplied by the base (TSC) frequency. Note that this only counts "non-halted" periods, so if the CPU was running at 1000 MHz half the time but halted the other half of the time (due to a frequency transition), you'd see 1000 MHz here, not 500 MHz.
 - `M/tsc-ratio` This shows the ration of the `MPERF` register to the TSC (time stamp counter) over the duration of the test. These counters count at the same rate, except that `MPERF` only counts "unhalted" cycles, while the TSC counts all cycles, so this ratio gives you an indication of the "lost" cycles due to halt events. A big source of halt events is frequency transitions in the turbo range: on my Skylake client CPU, any time another core starts up, the allowed turbo ratio changes, so the CPU halts for perhaps 20,000 cycles, so with moderate activity I often see ratios of 0.9 which means that 10% of the time my CPU is doing nothing. To get a "true" frequency, you should multiply this ratio by the `A/M-MHz` column, which would be the actual average frequency, counting halted periods as zero.       
