# Security Advisories — CVE Patch Log & Exemptions

> 安全漏洞处理记录:每次镜像扫描(CI 构建 / Docker Scout)发现的问题按版本记录在此。
> 修复类条目随版本发布;不适用条目给出豁免理由,重扫后仍有即属预期。
>
> This file tracks every CVE finding from container scans, per release: what was
> fixed (with the fixing version) and what is exempted (with the reason it does
> not apply to JNexus). Exempted findings are expected to reappear on every scan.

**处理原则 / Policy**

- 镜像内 JNexus 自身是 `CGO_ENABLED=0` 静态编译二进制,不链接 zlib/musl/openssl 等系统库;基础镜像层(apk 包)的漏洞只有运行时真的会执行到对应程序才算暴露。
- The JNexus binary is statically linked; base-image (apk) findings only count as exposed when the affected program is actually executed at runtime.
- 有修复版的一律随下个版本修复(升级依赖 / 基础镜像 / `apk upgrade`);上游确无修复版的逐条评估运行时可达性,不可达则记录豁免,等待上游修复后自然消除。
- Anything with an available fix is patched in the next release; findings without an upstream fix are assessed for runtime reachability — unreachable ones are exempted here until upstream ships a fix.

## 修复记录 / Patch log

| 版本 | 日期 | CVE | 严重度 | 组件 | 处理 |
| --- | --- | --- | --- | --- | --- |
| 2.110 | 2026-10-10 | CVE-2026-78663 | Critical 9.1 | golang.org/x/net | 升级 x/net v0.58.0 → **v0.60.0** |
| 2.110 | 2026-10-10 | CVE-2026-78669 / CVE-2026-78660 / CVE-2026-78659 | High 7.5 ×3 | golang.org/x/net | 同上(同一升级覆盖) |
| 2.109 | 2026-10-10 | CVE-2026-85091 | High 8.3 | apk / alpine/zlib 1.3.2-r0 | Dockerfile 运行层加 `apk upgrade --no-cache`,每次构建拉取分支安全修复(zlib → 1.3.2-r1);基础镜像其余有修复版的 CVE 同步清除 |
| 2.101 | 2026-10-09 | CVE-2026-78662 / CVE-2026-56855 / GO-2026-5932 | High 7.5 | golang.org/x/crypto | x/crypto v0.55.0 → v0.57.0 |
| 2.101 | 2026-10-09 | CVE-2026-2303 | Medium 6.9 | go.mongodb.org/mongo-driver | v1.13.1 → v1.17.10 |
| 2.101 | 2026-10-09 | CVE-2026-32952 | Medium 5.3 | github.com/Azure/go-ntlmssp | → v0.1.1 |
| 2.101 | 2026-10-09 | CVE-2025-60876 | Medium 6.5 | apk / alpine/busybox | 基础镜像 alpine:3.20 → 3.22(busybox 1.36 → 1.37,该 CVE 上游当时无修复版,见豁免) |

## 豁免说明 / Exemptions

| CVE | 严重度 | 组件 | 豁免理由 |
| --- | --- | --- | --- |
| CVE-2025-60876 | Medium 6.5 | apk / alpine/busybox(所有分支含 edge 均无修复版) | 漏洞点在 busybox 的 **wget 小程序**(HTTP 请求行可注入头)。镜像内 busybox 仅承担 `/bin/sh` 角色(entrypoint 脚本);wget 只在管理员显式配置 `JNEXUS_TRUST_CA_URL` 时于启动瞬间抓取内部 CA 证书,URL 由部署者控制,不存在攻击者可控的请求路径。EPSS ≈ 0.3%。上游(alpine)发布修复版后随 `apk upgrade` 自动消除。 |

> 豁免不等于忽略:每条豁免都注明了修复条件,上游发版后通过日常重建(基础镜像 + `apk upgrade`)自动收敛,无需改代码。
> Exempted ≠ ignored: each entry states its clearing condition; routine rebuilds pick up upstream fixes automatically.
