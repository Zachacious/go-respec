#!/usr/bin/env bash

# This script builds the respec binary and installs it to the local system
# for easy testing and debugging. It's a simple wrapper for 'make debug'.

set -euo pipefail

echo "🚀 Starting local debug build and install..."

# Ensure we are in the script's directory to find the Makefile correctly
cd "$(dirname "$0")/.."

make clean
make debug

echo "🎉 Done."

