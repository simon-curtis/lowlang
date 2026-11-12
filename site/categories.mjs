const prefixes = [
    ["std/archive", ["compression"]],
    ["std/compress", ["compression"]],
    ["std/crypto", ["crypto"]],
    ["std/hash", ["crypto"]],
    ["std/rand", ["crypto"]],
    ["std/tls", ["crypto", "net"]],
    ["std/webauthn", ["crypto", "web"]],
    ["std/ascii85", ["encoding"]],
    ["std/base32", ["encoding"]],
    ["std/base58", ["encoding"]],
    ["std/base64", ["encoding"]],
    ["std/charset", ["encoding", "text"]],
    ["std/unicode", ["encoding", "text"]],
    ["std/bson", ["formats"]],
    ["std/cbor", ["formats"]],
    ["std/csv", ["formats"]],
    ["std/dotenv", ["formats"]],
    ["std/ini", ["formats"]],
    ["std/json", ["formats"]],
    ["std/msgpack", ["formats"]],
    ["std/properties", ["formats"]],
    ["std/protobuf", ["formats"]],
    ["std/serde", ["formats"]],
    ["std/toml", ["formats"]],
    ["std/xml", ["formats"]],
    ["std/yaml", ["formats"]],
    ["std/net", ["net"]],
    ["std/socks", ["net"]],
    ["std/http", ["net", "web"]],
    ["std/mail", ["net"]],
    ["std/smtp", ["net"]],
    ["std/cdp", ["net"]],
    ["std/web", ["web"]],
    ["std/html", ["web", "formats"]],
    ["std/mime", ["web"]],
    ["std/multipart", ["web"]],
    ["std/template", ["web", "text"]],
    ["std/sql", ["database"]],
    ["std/orm", ["database"]],
    ["std/redis", ["database", "net"]],
    ["std/atomic", ["concurrency"]],
    ["std/cancel", ["concurrency"]],
    ["std/pipe", ["concurrency", "io"]],
    ["std/sync", ["concurrency"]],
    ["std/singleflight", ["concurrency"]],
    ["std/ratelimit", ["concurrency"]],
    ["std/retry", ["concurrency"]],
    ["std/io", ["io"]],
    ["std/sys", ["io", "os"]],
    ["std/file", ["io", "os"]],
    ["std/os", ["os"]],
    ["std/process", ["os"]],
    ["std/term", ["os"]],
    ["std/flag", ["os"]],
    ["std/path", ["os"]],
    ["std/time", ["time"]],
    ["std/text", ["text"]],
    ["std/regex", ["text"]],
    ["std/builder", ["text"]],
    ["std/big", ["math"]],
    ["std/math", ["math"]],
    ["std/simd", ["math"]],
    ["std/bits", ["data"]],
    ["std/container", ["data"]],
    ["std/maps", ["data"]],
    ["std/slices", ["data"]],
    ["std/uuid", ["data"]],
    ["std/semver", ["data"]],
    ["std/image", ["media"]],
    ["std/log", ["observability"]],
    ["std/otel", ["observability"]],
    ["std/testing", ["testing"]],
];

function covers(prefix, path) {
    return path === prefix || path.startsWith(`${prefix}/`);
}

export function categories(path) {
    const matches = prefixes.filter(([prefix]) => covers(prefix, path));
    const best = matches.sort(([a], [b]) => b.length - a.length)[0];
    return best ? best[1] : [];
}

export function orphaned(paths) {
    return prefixes.map(([prefix]) => prefix).filter((prefix) => !paths.some((path) => covers(prefix, path)));
}

export function all(paths) {
    return [...new Set(paths.flatMap(categories))].sort();
}
