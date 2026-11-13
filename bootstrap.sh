#!/bin/sh
set -eu

fail() {
    printf 'bootstrap: %s\n' "$1" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is required but not installed"
}

field() {
    value=$(sed -n "s/^$1 //p" "$root/bootstrap.txt" | head -n 1)
    [ -n "$value" ] || fail "bootstrap.txt has no $1 line"
    printf '%s' "$value"
}

hash_of() {
    sha256sum "$1" | cut -d ' ' -f 1
}

download() {
    url=$1
    target=$2
    mkdir -p "$(dirname "$target")" || fail "cannot create $(dirname "$target")"
    partial="$target.partial.$$"
    curl --fail --silent --show-error --location --output "$partial" "$url" || {
        rm -f "$partial"
        fail "download of $url failed"
    }
    mv "$partial" "$target"
}

root=$(cd "$(dirname "$0")" && pwd)
case ${1:-./low} in
    /*) output=${1:-./low} ;;
    *) output=$PWD/${1:-./low} ;;
esac

[ -f "$root/bootstrap.txt" ] || fail "$root/bootstrap.txt is missing"
need cmp
need cp
need mktemp
release=$(field release)
expected=$(field sha256)

if [ -n "${LOW_SEED:-}" ]; then
    seed=$LOW_SEED
    [ -f "$seed" ] || fail "LOW_SEED $seed is not a file"
else
    need curl
    cache=${XDG_CACHE_HOME:-$HOME/.cache}/lowlang/$release
    seed=$cache/low-linux-amd64
    if [ ! -f "$seed" ]; then
        printf 'bootstrap: downloading %s\n' "$release" >&2
        download "https://github.com/simon-curtis/lowlang/releases/download/$release/low-linux-amd64" "$seed"
    fi
fi

if [ "${LOW_SEED_UNCHECKED:-}" != 1 ]; then
    need sha256sum
    actual=$(hash_of "$seed")
    if [ "$actual" != "$expected" ]; then
        fail "$seed has sha256 $actual, bootstrap.txt expects $expected"
    fi
fi

chmod +x "$seed" || fail "cannot make $seed executable"

work=$(mktemp -d) || fail "cannot create a temporary directory"
trap 'rm -rf "$work"' EXIT INT TERM

cd "$root"
"$seed" build compiler/low -o "$work/stage1" || fail "the seed could not build stage 1"
"$work/stage1" build compiler/low -o "$work/stage2" || fail "stage 1 could not build stage 2"
"$work/stage2" build compiler/low -o "$work/stage3" || fail "stage 2 could not build stage 3"

if ! cmp -s "$work/stage2" "$work/stage3"; then
    fail "stage 2 and stage 3 differ"
fi

mkdir -p "$(dirname "$output")" || fail "cannot create $(dirname "$output")"
cp "$work/stage2" "$output" || fail "cannot write $output"
printf 'bootstrap: wrote %s\n' "$output" >&2
