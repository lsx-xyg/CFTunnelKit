#!/usr/bin/env bash
# 07a NSIS packaging (issue #3):
# cross-compile the Windows binary and build the NSIS installer, then
# generate checksums.txt for every artifact in build/bin.
#
# Prerequisites (Linux host):
#   sudo apt-get install -y nsis mingw-w64
set -euo pipefail
cd "$(dirname "$0")/.."

export PATH="$PATH:$HOME/gopath/bin"
export GOTOOLCHAIN=auto

wails build -platform windows/amd64 -nsis -clean "$@"

cd build/bin
sha256sum cftunnelkit-amd64-installer.exe cftunnelkit.exe > checksums.txt
echo "checksums.txt written:"
cat checksums.txt
