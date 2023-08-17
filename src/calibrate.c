//##########################################################################################################
// Copyright (C) 2020-2023 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause
//###########################################################################################################

#include <stdio.h>
#include <unistd.h>

typedef unsigned long long uint64_t;

static __inline__ uint64_t rdtsc_s(void)
{
  unsigned a=0;
  unsigned d=0;
  asm volatile("cpuid" ::: "%rax", "%rbx", "%rcx", "%rdx");
  asm volatile("rdtsc" : "=a" (a), "=d" (d)); 
  return ((unsigned long)a) | (((unsigned long)d) << 32); 
}

static __inline__ uint64_t rdtsc_e(void)
{
  unsigned a=0;
  unsigned d=0;
  asm volatile("rdtscp" : "=a" (a), "=d" (d)); 
  asm volatile("cpuid" ::: "%rax", "%rbx", "%rcx", "%rdx");
  return ((unsigned long)a) | (((unsigned long)d) << 32); 
}

unsigned Calibrate(void){
	uint64_t start=rdtsc_s();
	sleep(1);
	uint64_t end=rdtsc_e();

	uint64_t clocks_mhz= (end-start)/1000000;
	unsigned tsc_mhz = clocks_mhz;
	return tsc_mhz;
}

