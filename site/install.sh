#!/bin/sh
set -eu

platform="$(uname -s)/$(uname -m)"
case "$platform" in
Linux/x86_64 | Linux/amd64) ;;
*)
	echo "low: no build for $platform; low runs on linux/amd64 and windows/amd64" >&2
	exit 1
	;;
esac

dir="${LOW_INSTALL:-$HOME/.local/bin}"
mkdir -p "$dir"
download="$(mktemp "$dir/.low.XXXXXX")"
trap 'rm -f "$download"' EXIT
curl -fsSL https://github.com/simon-curtis/lowlang/releases/latest/download/low-linux-amd64 -o "$download"
chmod 755 "$download"
mv "$download" "$dir/low"
trap - EXIT

echo "installed $dir/low"
case ":$PATH:" in
*":$dir:"*) ;;
*) echo "add $dir to PATH to run low" ;;
esac
