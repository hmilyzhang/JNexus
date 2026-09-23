#!/bin/sh
# install-ca-trust.sh — install a CA certificate into the system trust store,
# distro-agnostic: Rocky / RHEL / CentOS (ca-trust), Ubuntu / Debian
# (update-ca-certificates), SUSE and Alpine also handled.
#
# Usage:
#   sudo sh install-ca-trust.sh <ca-cert-file-or-URL> [more certs...]
#   DRY_RUN=1 sh install-ca-trust.sh <cert>   # print the actions, change nothing
#
# The argument may be a local path or an http(s) URL (e.g. the internal CA
# served from localhost). Requires root for the real run.

set -e

DRY_RUN="${DRY_RUN:-0}"
note() { echo "[ca-trust] $*"; }
die() { echo "[ca-trust] ERROR: $*" >&2; exit 1; }

[ "$#" -ge 1 ] || die "usage: $0 <ca-cert-file-or-URL> [more certs...]"

fetch() {
    _src="$1" _out="$2"
    case "$_src" in
        http://*|https://*)
            if command -v wget >/dev/null 2>&1; then
                wget -q -T 15 -O "$_out" "$_src" || return 1
            elif command -v curl >/dev/null 2>&1; then
                curl -fsSL -m 15 -o "$_out" "$_src" || return 1
            else
                return 1
            fi
            ;;
        *)
            cp "$_src" "$_out" || return 1
            ;;
    esac
    return 0
}

install_one() {
    _src="$1"
    _tmp="$(mktemp -t jacert.XXXXXX)"
    if ! fetch "$_src" "$_tmp"; then
        rm -f "$_tmp"
        die "无法获取证书: $_src"
    fi
    # only accept PEM certificates — never trust an HTML error page
    if ! grep -q "BEGIN CERTIFICATE" "$_tmp"; then
        rm -f "$_tmp"
        die "内容不是 PEM 证书（缺少 BEGIN CERTIFICATE）: $_src"
    fi

    if command -v update-ca-trust >/dev/null 2>&1; then
        # RHEL / Rocky / CentOS / Fedora
        _dst="/etc/pki/ca-trust/source/anchors/$(basename "$_src" | sed 's/[^A-Za-z0-9._-]/_/g').pem"
        note "发行版检测: ca-trust (Rocky/RHEL 系)"
        note "cp $_tmp -> $_dst && update-ca-trust"
        if [ "$DRY_RUN" != "1" ]; then
            cp "$_tmp" "$_dst" || die "写入 $_dst 失败（需要 root？ca-certificates 包未安装？）"
            update-ca-trust || die "update-ca-trust 执行失败"
        fi
    elif command -v update-ca-certificates >/dev/null 2>&1; then
        # Debian / Ubuntu / SUSE / Alpine
        _dst="/usr/local/share/ca-certificates/$(basename "$_src" | sed 's/[^A-Za-z0-9._-]/_/g').crt"
        note "发行版检测: update-ca-certificates (Ubuntu/Debian 系)"
        note "cp $_tmp -> $_dst && update-ca-certificates"
        if [ "$DRY_RUN" != "1" ]; then
            cp "$_tmp" "$_dst" || die "写入 $_dst 失败（需要 root？ca-certificates 包未安装？）"
            update-ca-certificates || die "update-ca-certificates 执行失败"
        fi
    else
        rm -f "$_tmp"
        die "未找到 update-ca-trust / update-ca-certificates，无法识别发行版"
    fi
    rm -f "$_tmp"
    note "已信任: $_src"
}

for arg in "$@"; do
    install_one "$arg"
done

note "完成。容器场景无需本脚本：用 compose 挂载 /cacerts 或设置 JNEXUS_TRUST_CA_URL 即可。"
