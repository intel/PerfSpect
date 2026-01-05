# PerfSpect Build System

This directory contains the Docker-based build system for PerfSpect, which creates reproducible builds of the PerfSpect distribution packages for both x86_64 and aarch64 architectures.

## Overview

The build system uses a multi-stage approach:

1. **Tools Image** (`perfspect-tools:v1`): Contains pre-compiled external tools (perf, fio, etc.)
2. **Builder Image** (`perfspect-builder:v1`): Contains Go build environment with pre-installed dependencies
3. **Build Container**: Executes the actual build using the builder image

## Quick Start

```bash
# Build from the repository root
./builder/build.sh

# Force rebuild of tools (skip cache)
SKIP_TOOLS_CACHE=1 ./builder/build.sh
```

## Build Artifacts

The build produces the following artifacts in the `dist/` directory:

- `perfspect.tgz` - x86_64 distribution package
- `perfspect.tgz.md5.txt` - MD5 checksum for x86_64
- `perfspect-aarch64.tgz` - aarch64 distribution package
- `perfspect-aarch64.tgz.md5.txt` - MD5 checksum for aarch64
- `manifest.json` - Build metadata (version, commit, date)
- `oss_source.tgz` - Open source packages used in tools

## Tools Binary Caching
The build system uses caching to optimize build times.

**Purpose:** Skip expensive C compilation of external tools (perf, fio, etc.)

**Cache Location:**
- **Local:** `tools-cache/` directory (gitignored)
- **GitHub Actions:** GitHub's remote cache storage

**How It Works:**

### Local Development

**First Build:**
```
1. tools-cache/ doesn't exist
2. Build tools from source using tools/build.Dockerfile (~5-10 minutes)
   - Compiles perf from Linux kernel source
   - Compiles fio benchmark tool
   - Cross-compiles for both x86_64 and aarch64
3. Extract compiled binaries to tools-cache/
4. Build continues with perfspect-builder image
```

**Subsequent Builds:**
```
1. tools-cache/ exists and contains required files
2. Create minimal perfspect-tools:v1 image by copying from tools-cache/ (~5 seconds)
3. Skip building from source entirely
4. Build continues with perfspect-builder image
```

**Cache Invalidation:**
- Manual: `SKIP_TOOLS_CACHE=1 ./builder/build.sh`
- Manual: `make clean-tools-cache`
- Automatic: If required files are missing, cache is invalidated

### GitHub Actions

**First Run (cache miss):**
```
1. actions/cache attempts to restore tools-cache/ → MISS
2. Build tools from source using docker buildx with GHA layer cache
3. Extract binaries to tools-cache/
4. actions/cache automatically saves tools-cache/ to GitHub's cache
```

**Subsequent Runs (cache hit):**
```
1. actions/cache restores tools-cache/ from GitHub's cache → HIT
2. Workflow sets TOOLS_CACHE_HIT=true
3. Create minimal perfspect-tools:v1 image from cache
4. Skip building from source entirely
```

**Cache Invalidation:**
- Automatic: Cache key is based on hash of all files in the tools directory:
  ```
  key: perfspect-tools-binaries-${{ hashFiles('tools/**') }}
  ```
- When any of these files change, cache key changes and cache misses

## Build Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    builder/build.sh                         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
        ┌───────────────────────────────────┐
        │  Check tools-cache/ exists?       │
        └───────────────┬───────────────────┘
                        │
           ┌────────────┴────────────┐
           │                         │
          YES                       NO
           │                         │
           ▼                         ▼
    ┌─────────────┐          ┌──────────────┐
    │ Use cached  │          │ Build tools  │
    │ binaries    │          │ from source  │
    │             │          │              │
    │ COPY from   │          │ Docker build │
    │ tools-cache/│          │ (5-10 min)   │
    │ (~5 sec)    │          │              │
    └─────┬───────┘          └──────┬───────┘
          │                         │
          │                         ▼
          │                  ┌──────────────┐
          │                  │ Extract bins │
          │                  │ to tools-    │
          │                  │ cache/       │
          │                  └──────┬───────┘
          │                         │
          └─────────┬───────────────┘
                    ▼
         ┌─────────────────────┐
         │ perfspect-tools:v1  │
         │ (Docker image)      │
         └──────────┬──────────┘
                    ▼
         ┌─────────────────────┐
         │ Build perfspect-    │
         │ builder:v1          │
         │                     │
         │ - Copy tools from   │
         │   perfspect-tools   │
         └──────────┬──────────┘
                    ▼
         ┌─────────────────────┐
         │ Run build container │
         │                     │
         │ docker run          │
         │   perfspect-builder │
         │   make dist         │
         └──────────┬──────────┘
                    ▼
              ┌──────────┐
              │ dist/    │
              │ artifacts│
              └──────────┘
```

## Files

- `build.sh` - Main build orchestration script
- `build.Dockerfile` - Defines the perfspect-builder image with Go environment

## Environment Variables

- `TOOLS_CACHE_HIT=true` - Set by GitHub Actions when tools cache is restored
- `SKIP_TOOLS_CACHE=1` - Force rebuild tools from source, skip cache
- `GITHUB_ACTIONS` - Automatically set in GitHub Actions, enables buildx with GHA cache
