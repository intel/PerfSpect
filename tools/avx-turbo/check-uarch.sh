#!/bin/bash
set -e

# runs Intel SDE to check that ./avx-turbo works for every arch
all_arch=(   \
     p4p     \
     mrm     \
     pnr     \
     nhm     \
     wsm     \
     snb     \
     ivb     \
     hsw     \
     bdw     \
     slt     \
     slm     \
     glm     \
     tnt     \
     skl     \
     clx     \
     skx     \
     cnl     \
     icl     \
     icx     \
     knl     \
     knm     \
     future  \
)

for arch in "${all_arch[@]}"; do
    echo "Testing arch=$arch with SDE"
    sde64 -${arch} -- ./avx-turbo --max-threads=1
done