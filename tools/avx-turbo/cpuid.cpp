/*
 * cpuid.cpp
 */

#include "cpuid.hpp"

#include <string.h>

using std::uint8_t;
using std::uint32_t;


std::string cpuid_result::to_string() {
    std::string s;
    s += "eax = " + std::to_string(eax) + ", ";
    s += "ebx = " + std::to_string(ebx) + ", ";
    s += "ecx = " + std::to_string(ecx) + ", ";
    s += "edx = " + std::to_string(edx);
    return s;
}

uint32_t cpuid_highest_leaf_inner() {
    return cpuid(0).eax;
}

uint32_t cpuid_highest_leaf() {
    static uint32_t cached = cpuid_highest_leaf_inner();
    return cached;
}

cpuid_result cpuid(int leaf, int subleaf) {
    cpuid_result ret = {};
    asm ("cpuid"
            :
            "=a" (ret.eax),
            "=b" (ret.ebx),
            "=c" (ret.ecx),
            "=d" (ret.edx)
            :
            "a" (leaf),
            "c" (subleaf)
    );
    return ret;
}

cpuid_result cpuid(int leaf) {
    return cpuid(leaf, 0);
}

family_model gfm_inner() {
    auto cpuid1 = cpuid(1);
    family_model ret;
    ret.family   = (cpuid1.eax >> 8) & 0xF;
    ret.model    = (cpuid1.eax >> 4) & 0xF;
    ret.stepping = (cpuid1.eax     ) & 0xF;
    if (ret.family == 15) {
        ret.family += (cpuid1.eax >> 20) & 0xFF;  // extended family
    }
    if (ret.family == 15 || ret.family == 6) {
        ret.model += ((cpuid1.eax >> 16) & 0xF) << 4; // extended model
    }
    return ret;
}

family_model get_family_model() {
    static family_model cached_family_model = gfm_inner();
    return cached_family_model;
}

std::string get_brand_string() {
    auto check = cpuid(0x80000000);
    if (check.eax < 0x80000004) {
        return std::string("unkown (eax =") + std::to_string(check.eax) +")";
    }
    std::string ret;
    for (uint32_t eax : {0x80000002, 0x80000003, 0x80000004}) {
        char buf[17];
        auto fourchars = cpuid(eax);
        memcpy(buf +  0, &fourchars.eax, 4);
        memcpy(buf +  4, &fourchars.ebx, 4);
        memcpy(buf +  8, &fourchars.ecx, 4);
        memcpy(buf + 12, &fourchars.edx, 4);
        buf[16] = '\0';
        ret += buf;
    }
    return ret;
}

/* get bits [start:end] inclusive of the given value */
uint32_t get_bits(uint32_t value, int start, int end) {
    value >>= start;
    uint32_t mask = ((uint64_t)-1) << (end - start + 1);
    return value & ~mask;
}

/**
 * Get the shift amount for unique physical core IDs
 */
int get_smt_shift()
{
    if (cpuid_highest_leaf() < 0xb) {
        return -1;
    }
    uint32_t smtShift = -1u;
    for (uint32_t subleaf = 0; ; subleaf++) {
        cpuid_result leafb = cpuid(0xb, subleaf);
        uint32_t type  = get_bits(leafb.ecx, 8 ,15);
        if (!get_bits(leafb.ebx,0,15) || type == 0) {
            // done
            break;
        }
        if (type == 1) {
            // here's the value we are after: make sure we don't have more than one entry for
            // this type though!
            if (smtShift != -1u) {
                fprintf(stderr, "Warning: more than one level of type 1 in the x2APIC hierarchy");
            }
            smtShift = get_bits(leafb.eax, 0, 4);
        }
    }
    return smtShift;
}

