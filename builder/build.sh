#!/usr/bin/env bash
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# run this script from repo's root directory

set -ex

TAG=v1

# Determine if we're in GitHub Actions
if [ -n "$GITHUB_ACTIONS" ]; then
    # Use buildx with GitHub Actions cache
    CACHE_FROM="--cache-from type=gha,scope=perfspect-tools"
    CACHE_TO="--cache-to type=gha,mode=max,scope=perfspect-tools"
    BUILDER_CACHE_FROM="--cache-from type=gha,scope=perfspect-builder"
    BUILDER_CACHE_TO="--cache-to type=gha,mode=max,scope=perfspect-builder"
    BUILD_CMD="docker buildx build --load"
else
    # Local build without cache export
    CACHE_FROM=""
    CACHE_TO=""
    BUILDER_CACHE_FROM=""
    BUILDER_CACHE_TO=""
    BUILD_CMD="docker build"
fi

# Check if we can use cached binaries
USE_CACHE=""
if [ "$TOOLS_CACHE_HIT" = "true" ]; then
    # GitHub Actions cache hit
    USE_CACHE="true"
elif [ -z "$SKIP_TOOLS_CACHE" ] && [ -d "tools-cache/bin" ]; then
    # Local cache exists and not disabled
    echo "Found local tools cache in tools-cache/"
    echo "To force rebuild, run: SKIP_TOOLS_CACHE=1 builder/build.sh"
    USE_CACHE="true"
fi

# build tools image (or use cached binaries)
if [ "$USE_CACHE" = "true" ]; then
    echo "Using cached tool binaries, creating minimal tools image"
    # Create a minimal Dockerfile that packages the cached binaries
    cat > /tmp/cached-tools.Dockerfile << 'EOF'
FROM scratch AS output
COPY tools-cache/bin /bin
COPY tools-cache/oss_source.tgz /oss_source.tgz
COPY tools-cache/oss_source.tgz.md5 /oss_source.tgz.md5
EOF
    docker build -f /tmp/cached-tools.Dockerfile -t perfspect-tools:$TAG .
    rm /tmp/cached-tools.Dockerfile
else
    echo "Building tools from source"
    $BUILD_CMD -f tools/build.Dockerfile \
        $CACHE_FROM $CACHE_TO \
        --tag perfspect-tools:$TAG ./tools

    # Extract binaries for caching (both GitHub Actions and local)
    if [ -z "$SKIP_TOOLS_CACHE" ]; then
        echo "Extracting tool binaries to tools-cache/ for future builds"
        rm -rf tools-cache
        mkdir -p tools-cache
        # Use a helper container to extract files from the scratch-based image
        # Create a temporary Dockerfile that copies from the tools image
        cat > /tmp/extract-tools.Dockerfile << EOF
FROM perfspect-tools:$TAG AS source
FROM busybox:latest
COPY --from=source /bin /output/bin
COPY --from=source /oss_source.tgz /output/
COPY --from=source /oss_source.tgz.md5 /output/
CMD ["true"]
EOF
        # Build the extractor image
        docker build -q -f /tmp/extract-tools.Dockerfile -t perfspect-tools-extractor:$TAG . > /dev/null
        # Create container and extract files
        CONTAINER_ID=$(docker create perfspect-tools-extractor:$TAG)
        docker cp $CONTAINER_ID:/output/. tools-cache/
        docker rm $CONTAINER_ID > /dev/null
        # Cleanup
        docker rmi perfspect-tools-extractor:$TAG > /dev/null
        rm /tmp/extract-tools.Dockerfile
        echo "Tools cached locally. Next build will be faster!"
    fi
fi

# build the perfspect builder image
# Note: Always use regular docker build (not buildx) because it needs access to the
# locally built perfspect-tools:$TAG image. Buildx runs in an isolated builder context
# that doesn't have access to local Docker daemon images by default.
docker build -f builder/build.Dockerfile \
    --build-arg TAG=$TAG \
    --tag perfspect-builder:$TAG .

# build perfspect using the builder image
docker container run                                  \
    --volume "$(pwd)":/localrepo                      \
    -w /localrepo                                     \
    --rm                                              \
    perfspect-builder:$TAG                            \
    make dist