#!/usr/bin/env bash
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=$1
cnf=$here/ca.cnf
export CA_DIR=.
mkdir -p "$out"
openssl version > "$out/openssl.txt"

genkey() {
    openssl genpkey $KEYOPTS -out "$1" 2>/dev/null
}

ca_init() {
    mkdir -p "$1/certs"
    : > "$1/index.txt"
    echo 1000 > "$1/serial"
    echo 01 > "$1/crlnumber"
}

csr() {
    openssl req -new -key "$1" -subj "/CN=$2" -config "$cnf" -out "$3"
}

sign() {
    CA_DIR=$1 openssl ca -batch -config "$cnf" -extensions "$2" -in "$3" -out "$4" -notext -days "${5:-36500}" 2>/dev/null
}

issue() {
    genkey "$4.key"
    csr "$4.key" "$2" "$4.csr"
    sign "$1" "$3" "$4.csr" "$4.pem" "${5:-36500}"
}

make_ca() {
    ca_init "$4"
    genkey "$4/key.pem"
    csr "$4/key.pem" "$2" "$4/csr.pem"
    sign "$1" "$3" "$4/csr.pem" "$4/cert.pem"
}

gencrl() {
    local dir=$1 name=$2 section=${3:-crl_v2} days=${4:-36500}
    CA_DIR=$dir openssl ca -config "$cnf" -gencrl -crlexts "$section" -crldays "$days" -out "$dir/$name.pem" 2>/dev/null
    openssl crl -in "$dir/$name.pem" -outform DER -out "$dir/$name.crl"
}

request() {
    openssl ocsp ${5:-} -issuer "$1" -cert "$2" -reqout "$3" ${4:-} > /dev/null 2>&1
}

respond() {
    openssl ocsp -index "$1/index.txt" -CA "$1/cert.pem" -rsigner "$2" -rkey "$3" -reqin "$4" -respout "$5" -ndays "${6:-36500}" -nrequest 1 ${7:-} > /dev/null 2>&1
}

flip_last() {
    python3 -I -c 'import sys; d = bytearray(open(sys.argv[1], "rb").read()); d[-1] ^= 1; open(sys.argv[2], "wb").write(d)' "$1" "$2"
}

flip_at() {
    python3 -I -c 'import sys; d = bytearray(open(sys.argv[1], "rb").read()); d[int(sys.argv[3])] ^= 1; open(sys.argv[2], "wb").write(d)' "$1" "$2" "$3"
}

family() {
    local name=$1 f=$out/$1
    mkdir -p "$f"
    local root=$f/root inter=$f/inter inter2=$f/inter2 nocrl=$f/nocrl

    ca_init "$root"
    genkey "$root/key.pem"
    openssl req -new -x509 -key "$root/key.pem" -subj "/CN=Status Root $name" -days 36500 -config "$cnf" -extensions root_ext -out "$root/cert.pem"
    make_ca "$root" "Status Inter $name" inter_ext "$inter"
    make_ca "$root" "Status Second Inter $name" inter_ext "$inter2"
    make_ca "$root" "Status No CRL Sign $name" nocrlsign_ext "$nocrl"

    issue "$inter" localhost leaf_ext "$f/leaf"
    issue "$inter" localhost leaf_ext "$f/leaf_revoked"
    issue "$inter" localhost staple_ext "$f/leaf_staple"
    issue "$inter" localhost plain_ext "$f/leaf_plain"
    issue "$inter" localhost nocheck_ext "$f/leaf_nocheck"
    issue "$inter2" localhost leaf_ext "$f/leaf2"
    issue "$nocrl" localhost leaf_ext "$f/leaf_nocrl"
    issue "$inter" "Status Responder $name" responder_ext "$f/responder"
    issue "$inter" "Status Checked Responder $name" responder_checked_ext "$f/responder_checked"
    issue "$inter" "Status Responder Without Purpose $name" responder_noeku_ext "$f/responder_noeku"
    issue "$inter" "Status Expiring Responder $name" responder_ext "$f/responder_expiring" 1
    issue "$root" "Status Root Responder $name" responder_ext "$f/responder_root"

    genkey "$f/leaf_unknown.key"
    csr "$f/leaf_unknown.key" localhost "$f/leaf_unknown.csr"
    openssl x509 -req -in "$f/leaf_unknown.csr" -CA "$inter/cert.pem" -CAkey "$inter/key.pem" -set_serial 0x7001 -days 36500 -extfile "$cnf" -extensions leaf_ext -out "$f/leaf_unknown.pem" 2>/dev/null

    CA_DIR=$inter openssl ca -config "$cnf" -revoke "$f/leaf_revoked.pem" -crl_reason keyCompromise 2>/dev/null
    CA_DIR=$root openssl ca -config "$cnf" -revoke "$inter2/cert.pem" -crl_reason cACompromise 2>/dev/null

    gencrl "$inter" inter
    gencrl "$inter" inter_short crl_v2 1
    gencrl "$inter" inter_unknown crl_unknown
    gencrl "$inter" inter_idp_user crl_idp_user
    gencrl "$inter" inter_idp_ca crl_idp_ca
    gencrl "$inter" inter_idp_other crl_idp_other
    gencrl "$inter" inter_idp_indirect crl_idp_indirect
    gencrl "$inter" inter_idp_reasons crl_idp_reasons
    gencrl "$root" root
    gencrl "$nocrl" nocrl

    local empty=$f/emptydb
    ca_init "$empty"
    cp "$inter/cert.pem" "$inter/key.pem" "$empty/"
    gencrl "$empty" empty

    local v1=$f/v1db
    ca_init "$v1"
    cp "$inter/cert.pem" "$inter/key.pem" "$v1/"
    CA_DIR=$v1 openssl ca -name ca_v1 -config "$cnf" -revoke "$f/leaf_revoked.pem" 2>/dev/null
    CA_DIR=$v1 openssl ca -name ca_v1 -config "$cnf" -gencrl -out "$v1/v1.pem" 2>/dev/null
    openssl crl -in "$v1/v1.pem" -outform DER -out "$v1/v1.crl"

    local big=$f/bigdb
    ca_init "$big"
    cp "$inter/cert.pem" "$inter/key.pem" "$big/"
    for i in $(seq 0 999); do
        local reason=""
        if [ $((i % 7)) -eq 0 ]; then reason=",keyCompromise"; elif [ $((i % 5)) -eq 0 ]; then reason=",superseded"; elif [ $((i % 3)) -eq 0 ]; then reason=",certificateHold"; fi
        printf 'R\t491231235959Z\t261101%02d0000Z%s\t%06X\tunknown\t/CN=revoked-%d\n' $((i % 24)) "$reason" $((0x20000 + i)) "$i" >> "$big/index.txt"
    done
    CA_DIR=$big openssl ca -config "$cnf" -revoke "$f/leaf_revoked.pem" -crl_reason keyCompromise 2>/dev/null
    gencrl "$big" big

    local r=$f/ocsp
    mkdir -p "$r"
    request "$inter/cert.pem" "$f/leaf.pem" "$r/req_leaf.der" -no_nonce
    request "$inter/cert.pem" "$f/leaf.pem" "$r/req_leaf_nonce.der" -nonce
    request "$inter/cert.pem" "$f/leaf.pem" "$r/req_leaf_nonce_other.der" -nonce
    request "$inter/cert.pem" "$f/leaf.pem" "$r/req_leaf_sha256.der" -no_nonce -sha256
    request "$inter/cert.pem" "$f/leaf_revoked.pem" "$r/req_revoked.der" -no_nonce
    request "$inter/cert.pem" "$f/leaf_unknown.pem" "$r/req_unknown.der" -no_nonce
    request "$inter/cert.pem" "$f/leaf_staple.pem" "$r/req_staple.der" -no_nonce
    request "$root/cert.pem" "$inter/cert.pem" "$r/req_inter.der" -no_nonce
    request "$root/cert.pem" "$inter2/cert.pem" "$r/req_inter2.der" -no_nonce

    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_leaf.der" "$r/good_direct.der" 36500 -resp_no_certs
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_leaf.der" "$r/good_direct_certs.der" 36500
    respond "$inter" "$f/responder.pem" "$f/responder.key" "$r/req_leaf.der" "$r/good_delegated.der" 36500
    respond "$inter" "$f/responder.pem" "$f/responder.key" "$r/req_leaf.der" "$r/good_delegated_key.der" 36500 -resp_key_id
    respond "$inter" "$f/responder_checked.pem" "$f/responder_checked.key" "$r/req_leaf.der" "$r/good_delegated_checked.der" 36500
    respond "$inter" "$f/responder_noeku.pem" "$f/responder_noeku.key" "$r/req_leaf.der" "$r/good_no_purpose.der" 36500
    respond "$inter" "$f/responder_expiring.pem" "$f/responder_expiring.key" "$r/req_leaf.der" "$r/good_expiring_responder.der" 36500
    respond "$inter" "$f/responder_root.pem" "$f/responder_root.key" "$r/req_leaf.der" "$r/good_wrong_responder.der" 36500
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_leaf_nonce.der" "$r/good_nonce.der" 36500 -resp_no_certs
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_leaf_sha256.der" "$r/good_sha256.der" 36500 -resp_no_certs
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_leaf.der" "$r/good_short.der" 1 -resp_no_certs
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_revoked.der" "$r/revoked.der" 36500 -resp_no_certs
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_unknown.der" "$r/unknown.der" 36500 -resp_no_certs
    respond "$inter" "$inter/cert.pem" "$inter/key.pem" "$r/req_staple.der" "$r/staple.der" 36500 -resp_no_certs
    respond "$root" "$root/cert.pem" "$root/key.pem" "$r/req_inter.der" "$r/inter_good.der" 36500 -resp_no_certs
    respond "$root" "$root/cert.pem" "$root/key.pem" "$r/req_inter2.der" "$r/inter2_revoked.der" 36500 -resp_no_certs
    flip_last "$r/good_direct.der" "$r/good_tampered_signature.der"
    flip_at "$r/good_direct.der" "$r/good_tampered_data.der" 60
}

block() {
    printf -- '-----BEGIN %s-----\n' "$1"
    base64 -w64 "$2"
    printf -- '-----END %s-----\n' "$1"
}

cert_block() {
    openssl x509 -in "$2" -outform DER -out "$out/block.der"
    block "$1" "$out/block.der"
}

bundle() {
    local f=$out/$1 file=$out/$1.pem
    : > "$file"
    {
        cert_block ROOT "$f/root/cert.pem"
        cert_block INTER "$f/inter/cert.pem"
        cert_block INTER2 "$f/inter2/cert.pem"
        cert_block NOCRL "$f/nocrl/cert.pem"
        for c in leaf leaf_revoked leaf_unknown leaf_staple leaf_plain leaf_nocheck leaf2 leaf_nocrl responder responder_checked responder_noeku responder_expiring responder_root; do
            cert_block "$(echo "$c" | tr a-z A-Z)" "$f/$c.pem"
        done
        for c in inter inter_short inter_unknown inter_idp_user inter_idp_ca inter_idp_other inter_idp_indirect inter_idp_reasons; do
            block "CRL $(echo "$c" | tr a-z A-Z)" "$f/inter/$c.crl"
        done
        block "CRL ROOT" "$f/root/root.crl"
        block "CRL NOCRL" "$f/nocrl/nocrl.crl"
        block "CRL EMPTY" "$f/emptydb/empty.crl"
        block "CRL V1" "$f/v1db/v1.crl"
        if [ "$1" = p256 ]; then
            block "CRL BIG" "$f/bigdb/big.crl"
        fi
        for r in "$f"/ocsp/*.der; do
            block "OCSP $(basename "$r" .der | tr a-z A-Z)" "$r"
        done
    } >> "$file"
}

tls_bundles() {
    local f=$out/p256 t=$out/tls
    mkdir -p "$t"
    for c in leaf leaf_revoked leaf_staple; do
        {
            cat "$f/$c.pem"
            cat "$f/inter/cert.pem"
            cat "$f/$c.key"
        } > "$t/$c.pem"
    done
    cp "$f/root/cert.pem" "$t/ca.pem"
    {
        block "LEAF" "$f/ocsp/good_direct.der"
        block "REVOKED" "$f/ocsp/revoked.der"
        block "STAPLE" "$f/ocsp/staple.der"
        block "INTER" "$f/ocsp/inter_good.der"
        block "DELEGATED" "$f/ocsp/good_delegated.der"
        block "TAMPERED" "$f/ocsp/good_tampered_signature.der"
    } > "$t/ocsp.pem"
}

KEYOPTS="-algorithm RSA -pkeyopt rsa_keygen_bits:2048"
family rsa
KEYOPTS="-algorithm EC -pkeyopt ec_paramgen_curve:P-256"
family p256
bundle rsa
bundle p256
tls_bundles
rm -f "$out/block.der"
