#!/usr/bin/env bash

# This script builds the respec binary and installs it to the local system
# for easy testing and debugging. It's a simple wrapper for 'make debug'.

set -e

echo "🚀 Starting local debug build and install..."
make debug
echo "🎉 Done."