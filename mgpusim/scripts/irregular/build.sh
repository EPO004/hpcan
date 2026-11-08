#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
if [[ ${REBUILD_KERNELS:-0} == 1 ]]; then
  image=${IRREGULAR_COMPILER_IMAGE:-hpcan-irregular-llvm14:local}
  docker run --rm -u "$(id -u):$(id -g)" -v "$PWD/benchmarks/irregular:/work" -w /work "$image" bash -c '
    set -euo pipefail
    flags=(-x cl -target amdgcn-amd-amdhsa -mcpu=gfx803 -mcode-object-version=2 -nogpulib -cl-std=CL1.2 -O2 -ffp-contract=off)
    clang-14 "${flags[@]}" -S kernels.cl -o kernels.asm
    clang-14 "${flags[@]}" -c kernels.cl -o kernels.o
    ld.lld-14 -shared --no-rosegment kernels.o -o kernels.hsaco
  '
fi
go test ./benchmarks/irregular/... ./samples/irregular/... ./benchmarks/shoc/spmv/...
mkdir -p bin
go build -o bin/irregular ./samples/irregular
