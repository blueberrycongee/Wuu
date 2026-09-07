#!/bin/sh
# Seeds gn_out with the icudtl.dat that V8's mksnapshot needs at build time.
#
# The crates.io v8-150.4.0 package does not ship third_party/icu/common/icudtl.dat,
# and with the GN args pinned in .cargo/config.toml (icu_use_data_file=true,
# icu_use_stub_data=true) no ninja rule produces the file either. mksnapshot
# still initializes ICU from a data file next to the binary, so the from-source
# build fails at "run_mksnapshot_default" unless icudtl.dat exists in gn_out
# before ninja reaches that step. This file is exactly what the runtime loads at
# startup anyway: deno_core_icudata embeds the same bytes (include_bytes).
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
WORKSPACE_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
CARGO_HOME_DIR=${CARGO_HOME:-"$HOME/.cargo"}
ICU_VERSION=0.77.0

find_icu_data() {
  if [ -n "${ICU_DATA_FILE:-}" ] && [ -f "$ICU_DATA_FILE" ]; then
    echo "$ICU_DATA_FILE"
    return 0
  fi
  local exact
  exact=$(find "$CARGO_HOME_DIR/registry/src" \
    -path "*deno_core_icudata-$ICU_VERSION/src/icudtl.dat" -type f 2>/dev/null | head -1)
  if [ -n "$exact" ]; then
    echo "$exact"
    return 0
  fi
  echo "icudtl.dat not found under $CARGO_HOME_DIR/registry/src for deno_core_icudata-$ICU_VERSION." >&2
  echo "Run 'cd $WORKSPACE_DIR && cargo fetch' once to download dependencies, or set ICU_DATA_FILE." >&2
  return 1
}

ICU_DATA=$(find_icu_data)

for profile in debug release; do
  out="$WORKSPACE_DIR/target/$profile/gn_out"
  mkdir -p "$out"
  if [ -f "$out/icudtl.dat" ] && cmp -s "$ICU_DATA" "$out/icudtl.dat"; then
    echo "icudtl.dat already seeded in $out"
  else
    cp "$ICU_DATA" "$out/icudtl.dat"
    echo "seeded $out/icudtl.dat from $(basename "$ICU_DATA")"
  fi
done
