package metrics

// Copyright 2025 Google LLC.
// SPDX-License-Identifier: BSD-3-Clause

import (
	"perfspect/internal/report"
	"testing"
)

func TestParseLsCPUOutput(t *testing.T) {
	testCases := []struct {
		name                   string
		lscpuOutput            string
		expectedFamily         string
		expectedModel          string
		expectedStepping       string
		expectedSockets        int
		expectedCoresPerSocket int
		expectedThreadsPerCore int
		expectedError          error
	}{
		{
			name: "GCP C4A Axion",
			lscpuOutput: `Architecture:             aarch64
  CPU op-mode(s):         64-bit
  Byte Order:             Little Endian
CPU(s):                   72
  On-line CPU(s) list:    0-71
Vendor ID:                ARM
  Model name:             Neoverse-V2
    Model:                1
    Thread(s) per core:   1
    Core(s) per socket:   72
    Socket(s):            1
    Stepping:             r0p1
    BogoMIPS:             2000.00
    Flags:                fp asimd evtstrm aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm jscvt fcma lrcpc dcpop sha3 sm3 sm4 asimddp sha512 sve asim
                          dfhm dit uscat ilrcpc flagm sb paca pacg dcpodp sve2 sveaes svepmull svebitperm svesha3 svesm4 flagm2 frint svei8mm svebf16 i8mm bf16 dgh
                          rng bti
Caches (sum of all):
  L1d:                    4.5 MiB (72 instances)
  L1i:                    4.5 MiB (72 instances)
  L2:                     144 MiB (72 instances)
  L3:                     80 MiB (1 instance)
NUMA:
  NUMA node(s):           1
  NUMA node0 CPU(s):      0-71
Vulnerabilities:
  Gather data sampling:   Not affected
  Itlb multihit:          Not affected
  L1tf:                   Not affected
  Mds:                    Not affected
  Meltdown:               Not affected
  Mmio stale data:        Not affected
  Reg file data sampling: Not affected
  Retbleed:               Not affected
  Spec rstack overflow:   Not affected
  Spec store bypass:      Mitigation; Speculative Store Bypass disabled via prctl
  Spectre v1:             Mitigation; __user pointer sanitization
  Spectre v2:             Mitigation; CSV2, BHB
  Srbds:                  Not affected
  Tsx async abort:        Not affected`,
			expectedFamily:         "",
			expectedModel:          "1",
			expectedStepping:       "r0p1",
			expectedSockets:        1,
			expectedCoresPerSocket: 72,
			expectedThreadsPerCore: 1,
			expectedError:          nil,
		},
		{
			name: "Some x64 server",
			lscpuOutput: `Architecture:                         x86_64
CPU op-mode(s):                       32-bit, 64-bit
Byte Order:                           Little Endian
Address sizes:                        52 bits physical, 57 bits virtual
CPU(s):                               4
On-line CPU(s) list:                  0-3
Thread(s) per core:                   2
Core(s) per socket:                   2
Socket(s):                            1
NUMA node(s):                         1
Vendor ID:                            GenuineIntel
CPU family:                           6
Model:                                207
Model name:                           INTEL(R) XEON(R) PLATINUM 8581C CPU @ 2.30GHz
Stepping:                             2
CPU MHz:                              2300.000
BogoMIPS:                             4600.00
Hypervisor vendor:                    KVM
Virtualization type:                  full
L1d cache:                            96 KiB
L1i cache:                            64 KiB
L2 cache:                             4 MiB
L3 cache:                             260 MiB
NUMA node0 CPU(s):                    0-3
Vulnerability Gather data sampling:   Not affected
Vulnerability Itlb multihit:          Not affected
Vulnerability L1tf:                   Not affected
Vulnerability Mds:                    Not affected
Vulnerability Meltdown:               Not affected
Vulnerability Mmio stale data:        Not affected
Vulnerability Reg file data sampling: Not affected
Vulnerability Retbleed:               Not affected
Vulnerability Spec rstack overflow:   Not affected
Vulnerability Spec store bypass:      Mitigation; Speculative Store Bypass disabled via prctl and seccomp
Vulnerability Spectre v1:             Mitigation; usercopy/swapgs barriers and __user pointer sanitization
Vulnerability Spectre v2:             Mitigation; Enhanced / Automatic IBRS, IBPB conditional, RSB filling, PBRSB-eIBRS SW sequence
Vulnerability Srbds:                  Not affected
Vulnerability Tsx async abort:        Not affected
Flags:                                fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ss ht syscall nx pdpe1gb rdt
                                      scp lm constant_tsc arch_perfmon rep_good nopl xtopology nonstop_tsc cpuid tsc_known_freq pni pclmulqdq monitor ssse3 fma cx16
                                       pdcm pcid sse4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm abm 3dnowprefetch invpcid_single ssb
                                      d ibrs ibpb stibp ibrs_enhanced fsgsbase tsc_adjust bmi1 hle avx2 smep bmi2 erms invpcid rtm avx512f avx512dq rdseed adx smap
                                      avx512ifma clflushopt clwb avx512cd sha_ni avx512bw avx512vl xsaveopt xsavec xgetbv1 xsaves avx512_bf16 wbnoinvd arat avx512vb
                                      mi umip avx512_vbmi2 gfni vaes vpclmulqdq avx512_vnni avx512_bitalg avx512_vpopcntdq rdpid cldemote movdiri movdir64b fsrm md_
                                      clear serialize tsxldtrk arch_capabilities`,
			expectedFamily:         "6",
			expectedModel:          "207",
			expectedStepping:       "2",
			expectedSockets:        1,
			expectedCoresPerSocket: 2,
			expectedThreadsPerCore: 2,
			expectedError:          nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			family, model, stepping, sockets, cores, threads, _ := parseLsCPUOutput(tc.lscpuOutput)

			if family != tc.expectedFamily {
				t.Errorf("Expected family '%s', got '%s'", tc.expectedFamily, family)
			}
			if model != tc.expectedModel {
				t.Errorf("Expected model '%s', got '%s'", tc.expectedModel, model)
			}
			if stepping != tc.expectedStepping {
				t.Errorf("Expected stepping '%s', got '%s'", tc.expectedStepping, stepping)
			}
			if sockets != tc.expectedSockets {
				t.Errorf("Expected sockets %d, got %d", tc.expectedSockets, sockets)
			}
			if cores != tc.expectedCoresPerSocket {
				t.Errorf("Expected cores_per_socket %d, got %d", tc.expectedCoresPerSocket, cores)
			}
			if threads != tc.expectedThreadsPerCore {
				t.Errorf("Expected threads_per_core %d, got %d", tc.expectedThreadsPerCore, threads)
			}
		})
	}
}

func TestGetCPU(t *testing.T) {
	tests := []struct {
		name          string
		cpuFamily     string
		cpuModel      string
		cpuStepping   string
		wantMicroArch string
	}{
		{
			name:          "Neoverse V2",
			cpuFamily:     "",
			cpuModel:      "0xd4f",
			cpuStepping:   "r0p1",
			wantMicroArch: "Neoverse V2",
		},
		{
			name:          "Intel EMR - Family 6 Model 207 Stepping 2",
			cpuFamily:     "6",
			cpuModel:      "207",
			cpuStepping:   "2",
			wantMicroArch: "EMR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cpu, err := report.GetCPU(tc.cpuFamily, tc.cpuModel, tc.cpuStepping)

			if err != nil {
				t.Fatalf("GetCPU(%s) failed: %v", tc.name, err)
			}

			if cpu.MicroArchitecture != tc.wantMicroArch {
				t.Errorf("For %s, expected MicroArchitecture '%s', got '%s'", tc.name, tc.wantMicroArch, cpu.MicroArchitecture)
			}
		})
	}
}
