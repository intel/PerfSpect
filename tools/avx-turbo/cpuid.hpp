/*
 * cpuid.hpp
 */

#ifndef CPUID_HPP_
#define CPUID_HPP_

#include <cinttypes>
#include <string>

struct cpuid_result {
    std::uint32_t eax, ebx, ecx, edx;
    std::string to_string();
};

struct family_model {
    uint8_t family;
    uint8_t model;
    uint8_t stepping;
    std::string to_string() {
        std::string s;
        s += "family = " + std::to_string(family) + ", ";
        s += "model = " + std::to_string(model) + ", ";
        s += "stepping = " + std::to_string(stepping);
        return s;
    }
};


/** the highest supported leaf value */
uint32_t cpuid_highest_leaf();

/* return the CPUID result for querying the given leaf (EAX) and no subleaf (ECX=0) */
cpuid_result cpuid(int leaf);

/* return the CPUID result for querying the given leaf (EAX) and subleaf (ECX) */
cpuid_result cpuid(int leaf, int subleaf);

family_model get_family_model();

std::string get_brand_string();

int get_smt_shift();

/* get bits [start:end] inclusive of the given value */
uint32_t get_bits(uint32_t value, int start, int end);

#endif /* CPUID_HPP_ */
