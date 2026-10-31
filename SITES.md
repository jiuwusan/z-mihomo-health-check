# 海外测试站点池（200 个）

整理日期：2026-10-31（Asia/Shanghai）。用于后续 Mihomo 健康探查脚本抽样测试 HTTPS 可达性。

## 文件与格式

- [sites.txt](./sites.txt)：200 行，每行一个 HTTPS URL，无注释，可直接按行读取。
- [sites.json](./sites.json)：同一批 200 个站点，包含稳定编号、名称、分类、URL 和预期状态码范围。
- [probe-results.json](./probe-results.json)：本次本机探测快照，包括检查时间、最终 URL、HTTP 状态、重定向次数及耗时。

JSON 顶层是数组；每项的 `id` 为站点编号，`name` 为名称，`category` 为分类，`url` 为探查入口，`expectedStatus` 为字符串 `200-299`。后续重新筛选时应保留已有编号，新增站点继续递增。

## 筛选与验证

从 300 个候选站点中筛选，本次有 260 个通过网络检查，最终按类别抽取 200 个。优先选用海外机构、企业及国际开源项目的公开官网或文档入口，无需提交账户凭据。各类别数量如下。

| 类别 | 数量 |
| --- | ---: |
| 云服务与网络 | 17 |
| 开发平台与工具 | 17 |
| 编程语言与包管理 | 17 |
| 框架与数据库 | 17 |
| Linux与开源软件 | 17 |
| 标准与公益组织 | 17 |
| 高校与教育 | 17 |
| 科研与公共机构 | 17 |
| 新闻与文化 | 16 |
| 公共服务与应用 | 16 |
| 科学计算与文档 | 16 |
| 网络工具与备份 | 16 |
| 合计 | 200 |

“海外”表示站点主体或项目背景，不代表实际服务器物理位置。CDN、Anycast、地域跳转及共享基础设施会使请求落点变化；不同域名也可能共用服务商。已按原始与最终主机名及常见多级域名后缀去重，但未据此认定 200 个独立网络或运营主体。

检查使用 curl HEAD 请求，跟随至多 5 次 HTTPS 重定向，连接超时 5 秒，总超时 12 秒，开启 TLS 证书校验。入选项均为 curl 成功退出且最终返回 HTTP 2xx。超时、TLS 错误、HTTP 403/429/5xx 等结果未入选；未通过不等于站点已停服。使用原始入口 URL，以保留官网自身的规范化跳转。

快照只证明本次工作机网络视角下的响应，未指定目标 Mihomo 节点。curl 未显式指定代理，环境代理使用情况见 `proxyUsed`；该字段不能识别 TUN 或透明代理。耗时包含 DNS、连接、TLS 及重定向，不能作为节点延迟排名。

HEAD 验证不下载正文，不能确认页面是否为验证码或软错误页，也不验证登录、流媒体解锁、UDP 或吞吐量。接入自动切换前应通过实际候选节点复测，并按实际请求方法验证响应。

## 运行时抽样规则

常规探查默认每 **5 秒**执行一轮，每轮从完整的 200 个站点中**均匀随机抽取 1 个站点**测试主分组当前出口，不按类别轮询，也不全量遍历。各轮重新抽样，允许重复。连续失败达到面板配置阈值时触发 `🚀 节点选择` 的整组 delay 测试（默认阈值 **1 次**），随后按主分组原始成员顺序选择首个本轮测试成功的国家分组，不按延迟排序。详细流程见 [README.md](./README.md#3-节点自动切换策略)。

默认 5 秒间隔下，单个常规探查循环每分钟计划发起 12 次测试；单站长期平均约每 1,000 秒被常规探查选中一次。随机抽样不保证最短重访间隔，也不能保证不触发风控。故障触发的整组 delay、国家分组原生探查以及 HTTP 重定向均可能增加实际请求量，不能将每分钟 12 次视为总请求上限；自定义间隔时按实际配置计算频率。

Go 控制器已实现上述调度，间隔与连续失败阈值可配置。运行时默认测试超时 4 秒，串行执行并跳过忙碌时错过的调度周期；前述 12 秒超时仅属于站点池初筛快照。间隔改动后，站点长期平均被选择间隔为“站点数量 × 配置间隔”。

## 后续接入建议

- 记录常规探查、故障触发的整组测试，以及最终分组选择，便于区分站点故障和出口故障。
- 普通官网通常返回 200，不应统一要求 204。常规探查失败先触发整组测试，实际选择依据为整组测试结果与原始分组顺序。
- 这是供脚本读取的 URL 数据池。Mihomo 原生代理组的 `url` 和代理集合的 `health-check.url` 是单个 URL 字段，不能直接填入这 200 行。多站轮换需由后续探查逻辑处理。

字段说明参考 [Mihomo 代理组配置](https://wiki.metacubex.one/config/proxy-groups/) 与 [代理集合健康检查配置](https://wiki.metacubex.one/config/proxy-providers/)。站点 URL 及探测快照来自各站点本次实际 HTTPS 响应。

## 完整清单

### 云服务与网络

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-001 | Google | [https://www.google.com/](https://www.google.com/) |
| site-002 | Cloudflare | [https://www.cloudflare.com/](https://www.cloudflare.com/) |
| site-003 | Microsoft | [https://www.microsoft.com/](https://www.microsoft.com/) |
| site-004 | Apple | [https://www.apple.com/](https://www.apple.com/) |
| site-005 | Amazon Web Services | [https://aws.amazon.com/](https://aws.amazon.com/) |
| site-006 | Fastly | [https://www.fastly.com/](https://www.fastly.com/) |
| site-007 | DigitalOcean | [https://www.digitalocean.com/](https://www.digitalocean.com/) |
| site-008 | Linode | [https://www.linode.com/](https://www.linode.com/) |
| site-009 | Hetzner | [https://www.hetzner.com/](https://www.hetzner.com/) |
| site-010 | OVHcloud | [https://www.ovhcloud.com/](https://www.ovhcloud.com/) |
| site-011 | Scaleway | [https://www.scaleway.com/](https://www.scaleway.com/) |
| site-012 | Vercel | [https://vercel.com/](https://vercel.com/) |
| site-013 | Netlify | [https://www.netlify.com/](https://www.netlify.com/) |
| site-014 | Render | [https://render.com/](https://render.com/) |
| site-015 | Fly.io | [https://fly.io/](https://fly.io/) |
| site-016 | Heroku | [https://www.heroku.com/](https://www.heroku.com/) |
| site-017 | Tailscale | [https://tailscale.com/](https://tailscale.com/) |

### 开发平台与工具

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-018 | GitHub | [https://github.com/](https://github.com/) |
| site-019 | GitLab | [https://about.gitlab.com/](https://about.gitlab.com/) |
| site-020 | Codeberg | [https://codeberg.org/](https://codeberg.org/) |
| site-021 | SourceHut | [https://sourcehut.org/](https://sourcehut.org/) |
| site-022 | Bitbucket | [https://bitbucket.org/](https://bitbucket.org/) |
| site-023 | SourceForge | [https://sourceforge.net/](https://sourceforge.net/) |
| site-024 | Docker | [https://www.docker.com/](https://www.docker.com/) |
| site-025 | JetBrains | [https://www.jetbrains.com/](https://www.jetbrains.com/) |
| site-026 | Visual Studio Code | [https://code.visualstudio.com/](https://code.visualstudio.com/) |
| site-027 | Postman | [https://www.postman.com/](https://www.postman.com/) |
| site-028 | Insomnia | [https://insomnia.rest/](https://insomnia.rest/) |
| site-029 | Sentry | [https://sentry.io/](https://sentry.io/) |
| site-030 | Grafana | [https://grafana.com/](https://grafana.com/) |
| site-031 | Prometheus | [https://prometheus.io/](https://prometheus.io/) |
| site-032 | OpenTelemetry | [https://opentelemetry.io/](https://opentelemetry.io/) |
| site-033 | Ansible | [https://www.ansible.com/](https://www.ansible.com/) |
| site-034 | Puppet | [https://www.puppet.com/](https://www.puppet.com/) |

### 编程语言与包管理

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-035 | Python | [https://www.python.org/](https://www.python.org/) |
| site-036 | Go | [https://go.dev/](https://go.dev/) |
| site-037 | Node.js | [https://nodejs.org/](https://nodejs.org/) |
| site-038 | Ruby | [https://www.ruby-lang.org/](https://www.ruby-lang.org/) |
| site-039 | PHP | [https://www.php.net/](https://www.php.net/) |
| site-040 | Perl | [https://www.perl.org/](https://www.perl.org/) |
| site-041 | Lua | [https://www.lua.org/](https://www.lua.org/) |
| site-042 | Julia | [https://julialang.org/](https://julialang.org/) |
| site-043 | Haskell | [https://www.haskell.org/](https://www.haskell.org/) |
| site-044 | OCaml | [https://ocaml.org/](https://ocaml.org/) |
| site-045 | Elixir | [https://elixir-lang.org/](https://elixir-lang.org/) |
| site-046 | Erlang | [https://www.erlang.org/](https://www.erlang.org/) |
| site-047 | Scala | [https://www.scala-lang.org/](https://www.scala-lang.org/) |
| site-048 | Kotlin | [https://kotlinlang.org/](https://kotlinlang.org/) |
| site-049 | Swift | [https://www.swift.org/](https://www.swift.org/) |
| site-050 | Dart | [https://dart.dev/](https://dart.dev/) |
| site-051 | Zig | [https://ziglang.org/](https://ziglang.org/) |

### 框架与数据库

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-052 | React | [https://react.dev/](https://react.dev/) |
| site-053 | Vue | [https://vuejs.org/](https://vuejs.org/) |
| site-054 | Angular | [https://angular.dev/](https://angular.dev/) |
| site-055 | Svelte | [https://svelte.dev/](https://svelte.dev/) |
| site-056 | Next.js | [https://nextjs.org/](https://nextjs.org/) |
| site-057 | Astro | [https://astro.build/](https://astro.build/) |
| site-058 | Vite | [https://vite.dev/](https://vite.dev/) |
| site-059 | Webpack | [https://webpack.js.org/](https://webpack.js.org/) |
| site-060 | Rollup | [https://rollupjs.org/](https://rollupjs.org/) |
| site-061 | ESLint | [https://eslint.org/](https://eslint.org/) |
| site-062 | Prettier | [https://prettier.io/](https://prettier.io/) |
| site-063 | Django | [https://www.djangoproject.com/](https://www.djangoproject.com/) |
| site-064 | Flask | [https://flask.palletsprojects.com/](https://flask.palletsprojects.com/) |
| site-065 | FastAPI | [https://fastapi.tiangolo.com/](https://fastapi.tiangolo.com/) |
| site-066 | Ruby on Rails | [https://rubyonrails.org/](https://rubyonrails.org/) |
| site-067 | Laravel | [https://laravel.com/](https://laravel.com/) |
| site-068 | Spring | [https://spring.io/](https://spring.io/) |

### Linux与开源软件

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-069 | Linux Kernel | [https://www.kernel.org/](https://www.kernel.org/) |
| site-070 | Ubuntu | [https://ubuntu.com/](https://ubuntu.com/) |
| site-071 | Fedora | [https://fedoraproject.org/](https://fedoraproject.org/) |
| site-072 | Arch Linux | [https://archlinux.org/](https://archlinux.org/) |
| site-073 | openSUSE | [https://www.opensuse.org/](https://www.opensuse.org/) |
| site-074 | Alpine Linux | [https://alpinelinux.org/](https://alpinelinux.org/) |
| site-075 | NixOS | [https://nixos.org/](https://nixos.org/) |
| site-076 | Gentoo | [https://www.gentoo.org/](https://www.gentoo.org/) |
| site-077 | FreeBSD | [https://www.freebsd.org/](https://www.freebsd.org/) |
| site-078 | OpenBSD | [https://www.openbsd.org/](https://www.openbsd.org/) |
| site-079 | NetBSD | [https://www.netbsd.org/](https://www.netbsd.org/) |
| site-080 | GNU | [https://www.gnu.org/](https://www.gnu.org/) |
| site-081 | GNOME | [https://www.gnome.org/](https://www.gnome.org/) |
| site-082 | KDE | [https://kde.org/](https://kde.org/) |
| site-083 | Xfce | [https://xfce.org/](https://xfce.org/) |
| site-084 | LibreOffice | [https://www.libreoffice.org/](https://www.libreoffice.org/) |
| site-085 | Blender | [https://www.blender.org/](https://www.blender.org/) |

### 标准与公益组织

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-086 | W3C | [https://www.w3.org/](https://www.w3.org/) |
| site-087 | IETF | [https://www.ietf.org/](https://www.ietf.org/) |
| site-088 | RFC Editor | [https://www.rfc-editor.org/](https://www.rfc-editor.org/) |
| site-089 | IANA | [https://www.iana.org/](https://www.iana.org/) |
| site-090 | RIPE NCC | [https://www.ripe.net/](https://www.ripe.net/) |
| site-091 | ARIN | [https://www.arin.net/](https://www.arin.net/) |
| site-092 | APNIC | [https://www.apnic.net/](https://www.apnic.net/) |
| site-093 | LACNIC | [https://www.lacnic.net/](https://www.lacnic.net/) |
| site-094 | AFRINIC | [https://afrinic.net/](https://afrinic.net/) |
| site-095 | Electronic Frontier Foundation | [https://www.eff.org/](https://www.eff.org/) |
| site-096 | Free Software Foundation | [https://www.fsf.org/](https://www.fsf.org/) |
| site-097 | Apache Software Foundation | [https://www.apache.org/](https://www.apache.org/) |
| site-098 | Linux Foundation | [https://www.linuxfoundation.org/](https://www.linuxfoundation.org/) |
| site-099 | CNCF | [https://www.cncf.io/](https://www.cncf.io/) |
| site-100 | Open Source Initiative | [https://opensource.org/](https://opensource.org/) |
| site-101 | Creative Commons | [https://creativecommons.org/](https://creativecommons.org/) |
| site-102 | Wikimedia Foundation | [https://wikimediafoundation.org/](https://wikimediafoundation.org/) |

### 高校与教育

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-103 | MIT | [https://www.mit.edu/](https://www.mit.edu/) |
| site-104 | Harvard | [https://www.harvard.edu/](https://www.harvard.edu/) |
| site-105 | Caltech | [https://www.caltech.edu/](https://www.caltech.edu/) |
| site-106 | Yale | [https://www.yale.edu/](https://www.yale.edu/) |
| site-107 | Cornell | [https://www.cornell.edu/](https://www.cornell.edu/) |
| site-108 | Carnegie Mellon | [https://www.cmu.edu/](https://www.cmu.edu/) |
| site-109 | UC Berkeley | [https://www.berkeley.edu/](https://www.berkeley.edu/) |
| site-110 | Cambridge | [https://www.cam.ac.uk/](https://www.cam.ac.uk/) |
| site-111 | University of Edinburgh | [https://www.ed.ac.uk/](https://www.ed.ac.uk/) |
| site-112 | ETH Zurich | [https://ethz.ch/](https://ethz.ch/) |
| site-113 | EPFL | [https://www.epfl.ch/](https://www.epfl.ch/) |
| site-114 | Technical University of Munich | [https://www.tum.de/](https://www.tum.de/) |
| site-115 | University of Toronto | [https://www.utoronto.ca/](https://www.utoronto.ca/) |
| site-116 | University of British Columbia | [https://www.ubc.ca/](https://www.ubc.ca/) |
| site-117 | McGill | [https://www.mcgill.ca/](https://www.mcgill.ca/) |
| site-118 | University of Sydney | [https://www.sydney.edu.au/](https://www.sydney.edu.au/) |
| site-119 | National University of Singapore | [https://nus.edu.sg/](https://nus.edu.sg/) |

### 科研与公共机构

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-120 | NASA | [https://www.nasa.gov/](https://www.nasa.gov/) |
| site-121 | ESA | [https://www.esa.int/](https://www.esa.int/) |
| site-122 | CERN | [https://home.cern/](https://home.cern/) |
| site-123 | Fermilab | [https://www.fnal.gov/](https://www.fnal.gov/) |
| site-124 | NOAA | [https://www.noaa.gov/](https://www.noaa.gov/) |
| site-125 | NIST | [https://www.nist.gov/](https://www.nist.gov/) |
| site-126 | NSF | [https://www.nsf.gov/](https://www.nsf.gov/) |
| site-127 | PubMed | [https://pubmed.ncbi.nlm.nih.gov/](https://pubmed.ncbi.nlm.nih.gov/) |
| site-128 | arXiv | [https://arxiv.org/](https://arxiv.org/) |
| site-129 | Crossref | [https://www.crossref.org/](https://www.crossref.org/) |
| site-130 | ORCID | [https://orcid.org/](https://orcid.org/) |
| site-131 | Zenodo | [https://zenodo.org/](https://zenodo.org/) |
| site-132 | PLOS | [https://plos.org/](https://plos.org/) |
| site-133 | eLife | [https://elifesciences.org/](https://elifesciences.org/) |
| site-134 | Nature | [https://www.nature.com/](https://www.nature.com/) |
| site-135 | CSIRO | [https://www.csiro.au/](https://www.csiro.au/) |
| site-136 | Wellcome | [https://wellcome.org/](https://wellcome.org/) |

### 新闻与文化

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-137 | BBC | [https://www.bbc.com/](https://www.bbc.com/) |
| site-138 | The Guardian | [https://www.theguardian.com/](https://www.theguardian.com/) |
| site-139 | France 24 | [https://www.france24.com/](https://www.france24.com/) |
| site-140 | RFI | [https://www.rfi.fr/](https://www.rfi.fr/) |
| site-141 | NPR | [https://www.npr.org/](https://www.npr.org/) |
| site-142 | PBS | [https://www.pbs.org/](https://www.pbs.org/) |
| site-143 | CBC | [https://www.cbc.ca/](https://www.cbc.ca/) |
| site-144 | ABC Australia | [https://www.abc.net.au/](https://www.abc.net.au/) |
| site-145 | NHK World | [https://www3.nhk.or.jp/](https://www3.nhk.or.jp/) |
| site-146 | Al Jazeera | [https://www.aljazeera.com/](https://www.aljazeera.com/) |
| site-147 | SWI swissinfo | [https://www.swissinfo.ch/](https://www.swissinfo.ch/) |
| site-148 | The Conversation | [https://theconversation.com/](https://theconversation.com/) |
| site-149 | Louvre | [https://www.louvre.fr/](https://www.louvre.fr/) |
| site-150 | Rijksmuseum | [https://www.rijksmuseum.nl/](https://www.rijksmuseum.nl/) |
| site-151 | Tate | [https://www.tate.org.uk/](https://www.tate.org.uk/) |
| site-152 | Library of Congress | [https://www.loc.gov/](https://www.loc.gov/) |

### 公共服务与应用

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-153 | Wikipedia | [https://www.wikipedia.org/](https://www.wikipedia.org/) |
| site-154 | DuckDuckGo | [https://duckduckgo.com/](https://duckduckgo.com/) |
| site-155 | Brave Search | [https://search.brave.com/](https://search.brave.com/) |
| site-156 | Proton | [https://proton.me/](https://proton.me/) |
| site-157 | Signal | [https://signal.org/](https://signal.org/) |
| site-158 | Telegram | [https://telegram.org/](https://telegram.org/) |
| site-159 | Discord | [https://discord.com/](https://discord.com/) |
| site-160 | Slack | [https://slack.com/](https://slack.com/) |
| site-161 | Zoom | [https://www.zoom.com/](https://www.zoom.com/) |
| site-162 | Notion | [https://www.notion.com/](https://www.notion.com/) |
| site-163 | Obsidian | [https://obsidian.md/](https://obsidian.md/) |
| site-164 | Joplin | [https://joplinapp.org/](https://joplinapp.org/) |
| site-165 | Bitwarden | [https://bitwarden.com/](https://bitwarden.com/) |
| site-166 | 1Password | [https://1password.com/](https://1password.com/) |
| site-167 | KeePass | [https://keepass.info/](https://keepass.info/) |
| site-168 | Syncthing | [https://syncthing.net/](https://syncthing.net/) |

### 科学计算与文档

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-169 | Read the Docs | [https://about.readthedocs.com/](https://about.readthedocs.com/) |
| site-170 | Sphinx | [https://www.sphinx-doc.org/](https://www.sphinx-doc.org/) |
| site-171 | MkDocs | [https://www.mkdocs.org/](https://www.mkdocs.org/) |
| site-172 | Docusaurus | [https://docusaurus.io/](https://docusaurus.io/) |
| site-173 | Jekyll | [https://jekyllrb.com/](https://jekyllrb.com/) |
| site-174 | Eleventy | [https://www.11ty.dev/](https://www.11ty.dev/) |
| site-175 | Pelican | [https://getpelican.com/](https://getpelican.com/) |
| site-176 | Pandoc | [https://pandoc.org/](https://pandoc.org/) |
| site-177 | Quarto | [https://quarto.org/](https://quarto.org/) |
| site-178 | Typst | [https://typst.app/](https://typst.app/) |
| site-179 | LaTeX | [https://www.latex-project.org/](https://www.latex-project.org/) |
| site-180 | TeX Users Group | [https://tug.org/](https://tug.org/) |
| site-181 | CTAN | [https://ctan.org/](https://ctan.org/) |
| site-182 | MathJax | [https://www.mathjax.org/](https://www.mathjax.org/) |
| site-183 | KaTeX | [https://katex.org/](https://katex.org/) |
| site-184 | NumPy | [https://numpy.org/](https://numpy.org/) |

### 网络工具与备份

| 编号 | 站点 | 测试 URL |
| --- | --- | --- |
| site-185 | cURL | [https://curl.se/](https://curl.se/) |
| site-186 | OpenSSL | [https://openssl-library.org/](https://openssl-library.org/) |
| site-187 | OpenSSH | [https://www.openssh.com/](https://www.openssh.com/) |
| site-188 | OpenVPN | [https://openvpn.net/](https://openvpn.net/) |
| site-189 | strongSwan | [https://www.strongswan.org/](https://www.strongswan.org/) |
| site-190 | Wireshark | [https://www.wireshark.org/](https://www.wireshark.org/) |
| site-191 | Nmap | [https://nmap.org/](https://nmap.org/) |
| site-192 | HAProxy | [https://www.haproxy.org/](https://www.haproxy.org/) |
| site-193 | Caddy | [https://caddyserver.com/](https://caddyserver.com/) |
| site-194 | Traefik | [https://traefik.io/](https://traefik.io/) |
| site-195 | Envoy | [https://www.envoyproxy.io/](https://www.envoyproxy.io/) |
| site-196 | Squid | [https://www.squid-cache.org/](https://www.squid-cache.org/) |
| site-197 | Varnish | [https://varnish-cache.org/](https://varnish-cache.org/) |
| site-198 | PowerDNS | [https://www.powerdns.com/](https://www.powerdns.com/) |
| site-199 | NLnet Labs | [https://www.nlnetlabs.nl/](https://www.nlnetlabs.nl/) |
| site-200 | BIND / ISC | [https://www.isc.org/](https://www.isc.org/) |
