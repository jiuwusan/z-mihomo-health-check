# 三家美国出口交集精选探查站点池（470 个）

更新日期：2026-11-04（Asia/Shanghai）。从固定的 500 个候选地址中，取 E-IX 美国基线、固定 Kuromis 美国 02、固定 Sparkcloud 美国 02 三次测试的 488 个交集地址中，精选保留 470 个 HTTPS 地址，覆盖 470 个不同主机名。范围包含大厂、知名开源项目、高校与机构；新加坡测试和早先 Kuromis 自动切换分组的结果不参与入选判断。

## 筛选依据

- 每个地址在三个出口各测试三轮；超时 4 秒，expected=200-299，延迟须大于 0 且不超过 2000 ms，并发 2，轮间隔 30 秒。
- 除 delay API 成功及有效延迟外，每轮还须匹配 Mihomo 的逐地址历史记录，历史延迟同样须大于 0 且不超过 2000 ms；缺失记录或审计未通过均不入选。API HTTP 200 不等于目标网站返回 HTTP 200，不能仅依据 API 正延迟判断期望状态是否满足。
- E-IX 基线通过国家分组测试，检查点观察到实际节点为美国 02；Kuromis 和 Sparkcloud 直接指定美国 02 节点，未经过自动切换分组。三次测试分别为北京时间 16:26–16:43、18:10–18:26、17:51–18:06，属于不同时间的短窗口验证。
- 原 500 个地址经过 E-IX 筛选，存在基线筛选偏向；本记录不用于机场整体排名，也不承诺跨天、跨地区或长期可用率。

| 出口 | 500 个候选中三轮全部通过 |
| --- | ---: |
| E-IX 美国基线（观察到美国 02） | 500 |
| Kuromis 美国 02（固定） | 489 |
| Sparkcloud 美国 02（固定） | 498 |
| 三家交集 | 488 |
| 最终精选站点池 | 470 |

入选地址合计通过 4230 次探查及对应历史审计。500 个候选中，12 个因探查或历史审计未通过而排除，另 18 个虽通过测试，但因池规模限制被移除；两类原因分别保留在验证快照中。

从 488 个交集地址中，按九次探查的最大延迟升序选择前 470 个；相同最大延迟时，依次比较同一出口三轮的最大延迟范围、中位数延迟和 URL 字典序。单次延迟取 API 延迟与对应历史延迟的较大值；保留原清单顺序和编号。该规则优先降低已观察到的延迟及波动，不代表对长期稳定性的保证。

## 文件与运行规则

- [sites.txt](./sites.txt)：运行时嵌入的 470 行 HTTPS URL。
- [sites.json](./sites.json)：相同顺序的编号、名称、服务方、类型和 expectedStatus；已有地址保留原编号，新增地址从 site-567 起分配，不复用移除编号。
- [probe-results.json](./probe-results.json)：500 个候选的三家逐轮记录、历史状态审计与 selected 标记，以及源清单和入选池的 SHA-256，不包含服务连接凭据。

默认每 5 秒均匀随机抽取一个站点，每站长期平均约 2350 秒（39 分 10 秒）被选中一次，不保证最短重访间隔。整组测试、恢复检查和订阅自身探查会增加请求量。此次仅更新站点池，不修改实例的超时、间隔、阈值或节点切换逻辑。站点池在编译时嵌入，已有部署需重新构建并更新程序或镜像后生效。

入选池 SHA-256：`24d18be5980726e73857d7e8a10f4f66a64a51f5a0010006da02308259381033`。

## 当前清单

以下最大延迟仅统计各出口三轮 Mihomo delay 结果，不代表目标网站的 SLA。

| 编号 | 服务方 | 探查地址 | E-IX 最大延迟 | Kuromis 最大延迟 | Sparkcloud 最大延迟 |
| --- | --- | --- | ---: | ---: | ---: |
| site-504 | Google | [https://www.gstatic.com/generate_204](https://www.gstatic.com/generate_204) | 140 ms | 190 ms | 188 ms |
| site-506 | Apple | [https://captive.apple.com/hotspot-detect.html](https://captive.apple.com/hotspot-detect.html) | 135 ms | 185 ms | 186 ms |
| site-521 | IBM | [https://www.redhat.com/robots.txt](https://www.redhat.com/robots.txt) | 145 ms | 202 ms | 202 ms |
| site-509 | Microsoft | [https://raw.githubusercontent.com/github/gitignore/main/README.md](https://raw.githubusercontent.com/github/gitignore/main/README.md) | 137 ms | 473 ms | 186 ms |
| site-516 | Amazon | [https://www.aboutamazon.com/robots.txt](https://www.aboutamazon.com/robots.txt) | 143 ms | 482 ms | 198 ms |
| site-518 | Cloudflare | [https://developers.cloudflare.com/robots.txt](https://developers.cloudflare.com/robots.txt) | 792 ms | 220 ms | 226 ms |
| site-520 | SAP | [https://help.sap.com/robots.txt](https://help.sap.com/robots.txt) | 211 ms | 233 ms | 213 ms |
| site-517 | Meta | [https://www.facebook.com/robots.txt](https://www.facebook.com/robots.txt) | 883 ms | 198 ms | 193 ms |
| site-522 | Salesforce | [https://www.heroku.com/robots.txt](https://www.heroku.com/robots.txt) | 142 ms | 186 ms | 189 ms |
| site-523 | Dropbox | [https://help.dropbox.com/robots.txt](https://help.dropbox.com/robots.txt) | 157 ms | 207 ms | 219 ms |
| site-524 | Box | [https://www.box.com/robots.txt](https://www.box.com/robots.txt) | 166 ms | 210 ms | 223 ms |
| site-527 | Booking Holdings | [https://www.booking.com/robots.txt](https://www.booking.com/robots.txt) | 141 ms | 189 ms | 194 ms |
| site-525 | Stripe | [https://stripe.com/robots.txt](https://stripe.com/robots.txt) | 765 ms | 533 ms | 316 ms |
| site-529 | Broadcom | [https://www.broadcom.com/robots.txt](https://www.broadcom.com/robots.txt) | 156 ms | 208 ms | 204 ms |
| site-526 | Shopify | [https://help.shopify.com/robots.txt](https://help.shopify.com/robots.txt) | 209 ms | 326 ms | 350 ms |
| site-531 | Snowflake | [https://www.snowflake.com/robots.txt](https://www.snowflake.com/robots.txt) | 161 ms | 234 ms | 647 ms |
| site-536 | Intuit | [https://www.intuit.com/robots.txt](https://www.intuit.com/robots.txt) | 147 ms | 485 ms | 200 ms |
| site-532 | Datadog | [https://www.datadoghq.com/robots.txt](https://www.datadoghq.com/robots.txt) | 139 ms | 186 ms | 203 ms |
| site-537 | GitLab | [https://about.gitlab.com/robots.txt](https://about.gitlab.com/robots.txt) | 159 ms | 494 ms | 205 ms |
| site-534 | Zscaler | [https://www.zscaler.com/robots.txt](https://www.zscaler.com/robots.txt) | 433 ms | 206 ms | 221 ms |
| site-533 | CrowdStrike | [https://www.crowdstrike.com/robots.txt](https://www.crowdstrike.com/robots.txt) | 572 ms | 561 ms | 238 ms |
| site-535 | Okta | [https://www.okta.com/robots.txt](https://www.okta.com/robots.txt) | 159 ms | 490 ms | 218 ms |
| site-538 | Elastic | [https://www.elastic.co/robots.txt](https://www.elastic.co/robots.txt) | 169 ms | 515 ms | 221 ms |
| site-540 | DigitalOcean | [https://www.digitalocean.com/robots.txt](https://www.digitalocean.com/robots.txt) | 266 ms | 264 ms | 223 ms |
| site-539 | MongoDB | [https://www.mongodb.com/robots.txt](https://www.mongodb.com/robots.txt) | 139 ms | 187 ms | 188 ms |
| site-544 | Gen Digital | [https://www.avast.com/robots.txt](https://www.avast.com/robots.txt) | 217 ms | 228 ms | 228 ms |
| site-542 | Atlassian | [https://developer.atlassian.com/robots.txt](https://developer.atlassian.com/robots.txt) | 516 ms | 212 ms | 296 ms |
| site-543 | SUSE | [https://www.suse.com/robots.txt](https://www.suse.com/robots.txt) | 321 ms | 367 ms | 358 ms |
| site-549 | NVIDIA | [https://docs.nvidia.com/robots.txt](https://docs.nvidia.com/robots.txt) | 173 ms | 783 ms | 205 ms |
| site-548 | Oracle | [https://docs.oracle.com/robots.txt](https://docs.oracle.com/robots.txt) | 459 ms | 543 ms | 274 ms |
| site-546 | Kaspersky | [https://www.kaspersky.com/robots.txt](https://www.kaspersky.com/robots.txt) | 1021 ms | 374 ms | 339 ms |
| site-545 | ESET | [https://www.eset.com/robots.txt](https://www.eset.com/robots.txt) | 496 ms | 511 ms | 422 ms |
| site-547 | Adobe | [https://developer.adobe.com/robots.txt](https://developer.adobe.com/robots.txt) | 141 ms | 182 ms | 214 ms |
| site-555 | Apple | [https://www.apple.com/robots.txt](https://www.apple.com/robots.txt) | 540 ms | 486 ms | 190 ms |
| site-553 | Zoom | [https://developers.zoom.us/robots.txt](https://developers.zoom.us/robots.txt) | 141 ms | 190 ms | 196 ms |
| site-557 | Amazon | [https://aws.amazon.com/robots.txt](https://aws.amazon.com/robots.txt) | 173 ms | 227 ms | 267 ms |
| site-559 | Cloudflare | [https://www.cloudflare.com/robots.txt](https://www.cloudflare.com/robots.txt) | 251 ms | 757 ms | 222 ms |
| site-556 | Microsoft | [https://www.githubstatus.com/robots.txt](https://www.githubstatus.com/robots.txt) | 142 ms | 481 ms | 190 ms |
| site-558 | Meta | [https://www.whatsapp.com/robots.txt](https://www.whatsapp.com/robots.txt) | 417 ms | 626 ms | 279 ms |
| site-561 | IBM | [https://www.ibm.com/robots.txt](https://www.ibm.com/robots.txt) | 517 ms | 503 ms | 281 ms |
| site-560 | SAP | [https://www.sap.com/robots.txt](https://www.sap.com/robots.txt) | 436 ms | 664 ms | 427 ms |
| site-554 | Google | [https://www.google.com/generate_204](https://www.google.com/generate_204) | 140 ms | 187 ms | 190 ms |
| site-562 | Salesforce | [https://slack.com/robots.txt](https://slack.com/robots.txt) | 598 ms | 717 ms | 285 ms |
| site-563 | Docker | [https://www.docker.com/robots.txt](https://www.docker.com/robots.txt) | 141 ms | 190 ms | 194 ms |
| site-564 | Docker | [https://docs.docker.com/robots.txt](https://docs.docker.com/robots.txt) | 166 ms | 260 ms | 240 ms |
| site-566 | Docker | [https://download.docker.com/linux/static/stable/x86_64/](https://download.docker.com/linux/static/stable/x86_64/) | 141 ms | 188 ms | 195 ms |
| site-565 | Docker | [https://hub.docker.com/robots.txt](https://hub.docker.com/robots.txt) | 221 ms | 260 ms | 272 ms |
| site-567 | www.fastly.com | [https://www.fastly.com/robots.txt](https://www.fastly.com/robots.txt) | 850 ms | 186 ms | 190 ms |
| site-569 | www.netlify.com | [https://www.netlify.com/robots.txt](https://www.netlify.com/robots.txt) | 284 ms | 200 ms | 221 ms |
| site-570 | render.com | [https://render.com/robots.txt](https://render.com/robots.txt) | 225 ms | 233 ms | 253 ms |
| site-571 | www.hetzner.com | [https://www.hetzner.com/robots.txt](https://www.hetzner.com/robots.txt) | 283 ms | 329 ms | 349 ms |
| site-572 | fly.io | [https://fly.io/robots.txt](https://fly.io/robots.txt) | 315 ms | 192 ms | 203 ms |
| site-573 | tailscale.com | [https://tailscale.com/robots.txt](https://tailscale.com/robots.txt) | 260 ms | 643 ms | 263 ms |
| site-574 | github.com | [https://github.com/robots.txt](https://github.com/robots.txt) | 193 ms | 196 ms | 203 ms |
| site-575 | sourceforge.net | [https://sourceforge.net/robots.txt](https://sourceforge.net/robots.txt) | 192 ms | 314 ms | 208 ms |
| site-576 | bitbucket.org | [https://bitbucket.org/robots.txt](https://bitbucket.org/robots.txt) | 912 ms | 552 ms | 250 ms |
| site-577 | www.jetbrains.com | [https://www.jetbrains.com/robots.txt](https://www.jetbrains.com/robots.txt) | 721 ms | 191 ms | 192 ms |
| site-578 | codeberg.org | [https://codeberg.org/robots.txt](https://codeberg.org/robots.txt) | 297 ms | 351 ms | 349 ms |
| site-579 | code.visualstudio.com | [https://code.visualstudio.com/robots.txt](https://code.visualstudio.com/robots.txt) | 156 ms | 193 ms | 193 ms |
| site-580 | www.postman.com | [https://www.postman.com/robots.txt](https://www.postman.com/robots.txt) | 743 ms | 427 ms | 435 ms |
| site-581 | sentry.io | [https://sentry.io/robots.txt](https://sentry.io/robots.txt) | 751 ms | 551 ms | 254 ms |
| site-582 | insomnia.rest | [https://insomnia.rest/robots.txt](https://insomnia.rest/robots.txt) | 157 ms | 210 ms | 218 ms |
| site-583 | grafana.com | [https://grafana.com/robots.txt](https://grafana.com/robots.txt) | 190 ms | 237 ms | 254 ms |
| site-584 | prometheus.io | [https://prometheus.io/robots.txt](https://prometheus.io/robots.txt) | 172 ms | 532 ms | 219 ms |
| site-585 | www.microsoft.com | [https://www.microsoft.com/robots.txt](https://www.microsoft.com/robots.txt) | 520 ms | 490 ms | 224 ms |
| site-586 | opentelemetry.io | [https://opentelemetry.io/robots.txt](https://opentelemetry.io/robots.txt) | 508 ms | 207 ms | 205 ms |
| site-587 | www.puppet.com | [https://www.puppet.com/robots.txt](https://www.puppet.com/robots.txt) | 495 ms | 191 ms | 199 ms |
| site-588 | www.python.org | [https://www.python.org/robots.txt](https://www.python.org/robots.txt) | 139 ms | 480 ms | 186 ms |
| site-589 | nodejs.org | [https://nodejs.org/robots.txt](https://nodejs.org/robots.txt) | 183 ms | 219 ms | 215 ms |
| site-590 | go.dev | [https://go.dev/robots.txt](https://go.dev/robots.txt) | 204 ms | 662 ms | 250 ms |
| site-591 | www.ruby-lang.org | [https://www.ruby-lang.org/robots.txt](https://www.ruby-lang.org/robots.txt) | 140 ms | 186 ms | 194 ms |
| site-592 | www.php.net | [https://www.php.net/robots.txt](https://www.php.net/robots.txt) | 373 ms | 476 ms | 194 ms |
| site-593 | www.perl.org | [https://www.perl.org/robots.txt](https://www.perl.org/robots.txt) | 840 ms | 187 ms | 195 ms |
| site-594 | www.haskell.org | [https://www.haskell.org/robots.txt](https://www.haskell.org/robots.txt) | 200 ms | 249 ms | 265 ms |
| site-595 | ocaml.org | [https://ocaml.org/robots.txt](https://ocaml.org/robots.txt) | 289 ms | 331 ms | 333 ms |
| site-596 | elixir-lang.org | [https://elixir-lang.org/robots.txt](https://elixir-lang.org/robots.txt) | 139 ms | 189 ms | 207 ms |
| site-597 | react.dev | [https://react.dev/robots.txt](https://react.dev/robots.txt) | 166 ms | 519 ms | 203 ms |
| site-598 | kotlinlang.org | [https://kotlinlang.org/robots.txt](https://kotlinlang.org/robots.txt) | 588 ms | 989 ms | 653 ms |
| site-599 | www.swift.org | [https://www.swift.org/robots.txt](https://www.swift.org/robots.txt) | 149 ms | 482 ms | 195 ms |
| site-600 | vuejs.org | [https://vuejs.org/robots.txt](https://vuejs.org/robots.txt) | 150 ms | 196 ms | 201 ms |
| site-601 | angular.dev | [https://angular.dev/robots.txt](https://angular.dev/robots.txt) | 138 ms | 182 ms | 185 ms |
| site-602 | svelte.dev | [https://svelte.dev/robots.txt](https://svelte.dev/robots.txt) | 153 ms | 489 ms | 204 ms |
| site-603 | astro.build | [https://astro.build/robots.txt](https://astro.build/robots.txt) | 171 ms | 496 ms | 228 ms |
| site-604 | www.scala-lang.org | [https://www.scala-lang.org/robots.txt](https://www.scala-lang.org/robots.txt) | 187 ms | 213 ms | 211 ms |
| site-605 | webpack.js.org | [https://webpack.js.org/robots.txt](https://webpack.js.org/robots.txt) | 218 ms | 222 ms | 222 ms |
| site-606 | eslint.org | [https://eslint.org/robots.txt](https://eslint.org/robots.txt) | 156 ms | 198 ms | 209 ms |
| site-607 | fastapi.tiangolo.com | [https://fastapi.tiangolo.com/robots.txt](https://fastapi.tiangolo.com/robots.txt) | 172 ms | 227 ms | 239 ms |
| site-608 | www.djangoproject.com | [https://www.djangoproject.com/robots.txt](https://www.djangoproject.com/robots.txt) | 140 ms | 184 ms | 198 ms |
| site-609 | rubyonrails.org | [https://rubyonrails.org/robots.txt](https://rubyonrails.org/robots.txt) | 141 ms | 178 ms | 190 ms |
| site-610 | spring.io | [https://spring.io/robots.txt](https://spring.io/robots.txt) | 506 ms | 196 ms | 199 ms |
| site-611 | flask.palletsprojects.com | [https://flask.palletsprojects.com/robots.txt](https://flask.palletsprojects.com/robots.txt) | 159 ms | 502 ms | 226 ms |
| site-612 | fedoraproject.org | [https://fedoraproject.org/robots.txt](https://fedoraproject.org/robots.txt) | 730 ms | 213 ms | 779 ms |
| site-613 | laravel.com | [https://laravel.com/robots.txt](https://laravel.com/robots.txt) | 519 ms | 497 ms | 211 ms |
| site-614 | archlinux.org | [https://archlinux.org/robots.txt](https://archlinux.org/robots.txt) | 294 ms | 347 ms | 347 ms |
| site-615 | www.freebsd.org | [https://www.freebsd.org/robots.txt](https://www.freebsd.org/robots.txt) | 142 ms | 194 ms | 199 ms |
| site-616 | nixos.org | [https://nixos.org/robots.txt](https://nixos.org/robots.txt) | 154 ms | 490 ms | 199 ms |
| site-618 | www.netbsd.org | [https://www.netbsd.org/robots.txt](https://www.netbsd.org/robots.txt) | 136 ms | 479 ms | 217 ms |
| site-619 | www.gentoo.org | [https://www.gentoo.org/robots.txt](https://www.gentoo.org/robots.txt) | 242 ms | 187 ms | 189 ms |
| site-620 | www.w3.org | [https://www.w3.org/robots.txt](https://www.w3.org/robots.txt) | 519 ms | 196 ms | 206 ms |
| site-621 | kde.org | [https://kde.org/robots.txt](https://kde.org/robots.txt) | 297 ms | 624 ms | 367 ms |
| site-622 | www.blender.org | [https://www.blender.org/robots.txt](https://www.blender.org/robots.txt) | 494 ms | 199 ms | 201 ms |
| site-623 | www.ietf.org | [https://www.ietf.org/robots.txt](https://www.ietf.org/robots.txt) | 155 ms | 221 ms | 197 ms |
| site-624 | xfce.org | [https://xfce.org/robots.txt](https://xfce.org/robots.txt) | 266 ms | 321 ms | 337 ms |
| site-625 | www.rfc-editor.org | [https://www.rfc-editor.org/robots.txt](https://www.rfc-editor.org/robots.txt) | 276 ms | 585 ms | 312 ms |
| site-626 | www.iana.org | [https://www.iana.org/robots.txt](https://www.iana.org/robots.txt) | 145 ms | 204 ms | 204 ms |
| site-627 | www.apnic.net | [https://www.apnic.net/robots.txt](https://www.apnic.net/robots.txt) | 509 ms | 201 ms | 200 ms |
| site-628 | www.arin.net | [https://www.arin.net/robots.txt](https://www.arin.net/robots.txt) | 196 ms | 262 ms | 704 ms |
| site-629 | www.apache.org | [https://www.apache.org/robots.txt](https://www.apache.org/robots.txt) | 514 ms | 484 ms | 188 ms |
| site-630 | www.eff.org | [https://www.eff.org/robots.txt](https://www.eff.org/robots.txt) | 139 ms | 523 ms | 190 ms |
| site-631 | www.lacnic.net | [https://www.lacnic.net/robots.txt](https://www.lacnic.net/robots.txt) | 309 ms | 352 ms | 366 ms |
| site-632 | www.linuxfoundation.org | [https://www.linuxfoundation.org/robots.txt](https://www.linuxfoundation.org/robots.txt) | 654 ms | 226 ms | 209 ms |
| site-633 | opensource.org | [https://opensource.org/robots.txt](https://opensource.org/robots.txt) | 166 ms | 235 ms | 211 ms |
| site-635 | afrinic.net | [https://afrinic.net/robots.txt](https://afrinic.net/robots.txt) | 398 ms | 466 ms | 483 ms |
| site-636 | wikimediafoundation.org | [https://wikimediafoundation.org/robots.txt](https://wikimediafoundation.org/robots.txt) | 199 ms | 478 ms | 188 ms |
| site-637 | www.mit.edu | [https://www.mit.edu/robots.txt](https://www.mit.edu/robots.txt) | 209 ms | 212 ms | 210 ms |
| site-638 | www.yale.edu | [https://www.yale.edu/robots.txt](https://www.yale.edu/robots.txt) | 138 ms | 212 ms | 190 ms |
| site-639 | www.harvard.edu | [https://www.harvard.edu/robots.txt](https://www.harvard.edu/robots.txt) | 139 ms | 487 ms | 193 ms |
| site-640 | www.cornell.edu | [https://www.cornell.edu/robots.txt](https://www.cornell.edu/robots.txt) | 139 ms | 206 ms | 196 ms |
| site-641 | www.caltech.edu | [https://www.caltech.edu/robots.txt](https://www.caltech.edu/robots.txt) | 237 ms | 224 ms | 224 ms |
| site-642 | www.cmu.edu | [https://www.cmu.edu/robots.txt](https://www.cmu.edu/robots.txt) | 901 ms | 251 ms | 249 ms |
| site-644 | www.ed.ac.uk | [https://www.ed.ac.uk/robots.txt](https://www.ed.ac.uk/robots.txt) | 145 ms | 523 ms | 188 ms |
| site-645 | www.cam.ac.uk | [https://www.cam.ac.uk/robots.txt](https://www.cam.ac.uk/robots.txt) | 140 ms | 190 ms | 194 ms |
| site-646 | www.ubc.ca | [https://www.ubc.ca/robots.txt](https://www.ubc.ca/robots.txt) | 141 ms | 187 ms | 289 ms |
| site-647 | www.mcgill.ca | [https://www.mcgill.ca/robots.txt](https://www.mcgill.ca/robots.txt) | 149 ms | 520 ms | 208 ms |
| site-648 | www.tum.de | [https://www.tum.de/robots.txt](https://www.tum.de/robots.txt) | 330 ms | 388 ms | 384 ms |
| site-649 | www.nasa.gov | [https://www.nasa.gov/robots.txt](https://www.nasa.gov/robots.txt) | 138 ms | 203 ms | 187 ms |
| site-650 | www.sydney.edu.au | [https://www.sydney.edu.au/robots.txt](https://www.sydney.edu.au/robots.txt) | 134 ms | 485 ms | 201 ms |
| site-651 | www.esa.int | [https://www.esa.int/robots.txt](https://www.esa.int/robots.txt) | 137 ms | 472 ms | 190 ms |
| site-652 | home.cern | [https://home.cern/robots.txt](https://home.cern/robots.txt) | 289 ms | 649 ms | 349 ms |
| site-653 | www.nist.gov | [https://www.nist.gov/robots.txt](https://www.nist.gov/robots.txt) | 174 ms | 227 ms | 244 ms |
| site-654 | www.noaa.gov | [https://www.noaa.gov/robots.txt](https://www.noaa.gov/robots.txt) | 141 ms | 490 ms | 215 ms |
| site-655 | arxiv.org | [https://arxiv.org/robots.txt](https://arxiv.org/robots.txt) | 251 ms | 434 ms | 384 ms |
| site-656 | www.nsf.gov | [https://www.nsf.gov/robots.txt](https://www.nsf.gov/robots.txt) | 143 ms | 200 ms | 193 ms |
| site-657 | www.crossref.org | [https://www.crossref.org/robots.txt](https://www.crossref.org/robots.txt) | 210 ms | 245 ms | 249 ms |
| site-658 | pubmed.ncbi.nlm.nih.gov | [https://pubmed.ncbi.nlm.nih.gov/robots.txt](https://pubmed.ncbi.nlm.nih.gov/robots.txt) | 202 ms | 550 ms | 256 ms |
| site-659 | orcid.org | [https://orcid.org/robots.txt](https://orcid.org/robots.txt) | 161 ms | 856 ms | 224 ms |
| site-660 | elifesciences.org | [https://elifesciences.org/robots.txt](https://elifesciences.org/robots.txt) | 137 ms | 189 ms | 186 ms |
| site-661 | www.nature.com | [https://www.nature.com/robots.txt](https://www.nature.com/robots.txt) | 177 ms | 235 ms | 243 ms |
| site-662 | plos.org | [https://plos.org/robots.txt](https://plos.org/robots.txt) | 271 ms | 291 ms | 280 ms |
| site-663 | wellcome.org | [https://wellcome.org/robots.txt](https://wellcome.org/robots.txt) | 141 ms | 515 ms | 190 ms |
| site-664 | www.rfi.fr | [https://www.rfi.fr/robots.txt](https://www.rfi.fr/robots.txt) | 151 ms | 205 ms | 195 ms |
| site-665 | www.npr.org | [https://www.npr.org/robots.txt](https://www.npr.org/robots.txt) | 145 ms | 204 ms | 204 ms |
| site-666 | www.pbs.org | [https://www.pbs.org/robots.txt](https://www.pbs.org/robots.txt) | 361 ms | 570 ms | 393 ms |
| site-667 | www.france24.com | [https://www.france24.com/robots.txt](https://www.france24.com/robots.txt) | 146 ms | 219 ms | 225 ms |
| site-668 | www.csiro.au | [https://www.csiro.au/robots.txt](https://www.csiro.au/robots.txt) | 291 ms | 803 ms | 371 ms |
| site-669 | www3.nhk.or.jp | [https://www3.nhk.or.jp/robots.txt](https://www3.nhk.or.jp/robots.txt) | 151 ms | 119 ms | 211 ms |
| site-670 | www.abc.net.au | [https://www.abc.net.au/robots.txt](https://www.abc.net.au/robots.txt) | 506 ms | 209 ms | 207 ms |
| site-671 | www.aljazeera.com | [https://www.aljazeera.com/robots.txt](https://www.aljazeera.com/robots.txt) | 164 ms | 493 ms | 206 ms |
| site-672 | www.swissinfo.ch | [https://www.swissinfo.ch/robots.txt](https://www.swissinfo.ch/robots.txt) | 169 ms | 197 ms | 192 ms |
| site-673 | theconversation.com | [https://theconversation.com/robots.txt](https://theconversation.com/robots.txt) | 752 ms | 483 ms | 194 ms |
| site-674 | www.tate.org.uk | [https://www.tate.org.uk/robots.txt](https://www.tate.org.uk/robots.txt) | 516 ms | 201 ms | 428 ms |
| site-675 | www.rijksmuseum.nl | [https://www.rijksmuseum.nl/robots.txt](https://www.rijksmuseum.nl/robots.txt) | 418 ms | 651 ms | 405 ms |
| site-676 | duckduckgo.com | [https://duckduckgo.com/robots.txt](https://duckduckgo.com/robots.txt) | 256 ms | 367 ms | 250 ms |
| site-677 | www.louvre.fr | [https://www.louvre.fr/robots.txt](https://www.louvre.fr/robots.txt) | 525 ms | 315 ms | 311 ms |
| site-678 | proton.me | [https://proton.me/robots.txt](https://proton.me/robots.txt) | 337 ms | 348 ms | 357 ms |
| site-679 | discord.com | [https://discord.com/robots.txt](https://discord.com/robots.txt) | 144 ms | 190 ms | 203 ms |
| site-680 | www.loc.gov | [https://www.loc.gov/robots.txt](https://www.loc.gov/robots.txt) | 231 ms | 478 ms | 197 ms |
| site-681 | www.notion.com | [https://www.notion.com/robots.txt](https://www.notion.com/robots.txt) | 839 ms | 330 ms | 230 ms |
| site-682 | www.zoom.com | [https://www.zoom.com/robots.txt](https://www.zoom.com/robots.txt) | 192 ms | 222 ms | 217 ms |
| site-683 | obsidian.md | [https://obsidian.md/robots.txt](https://obsidian.md/robots.txt) | 146 ms | 202 ms | 202 ms |
| site-684 | bitwarden.com | [https://bitwarden.com/robots.txt](https://bitwarden.com/robots.txt) | 304 ms | 211 ms | 200 ms |
| site-685 | about.readthedocs.com | [https://about.readthedocs.com/robots.txt](https://about.readthedocs.com/robots.txt) | 181 ms | 207 ms | 228 ms |
| site-686 | www.sphinx-doc.org | [https://www.sphinx-doc.org/robots.txt](https://www.sphinx-doc.org/robots.txt) | 686 ms | 231 ms | 224 ms |
| site-687 | 1password.com | [https://1password.com/robots.txt](https://1password.com/robots.txt) | 482 ms | 254 ms | 449 ms |
| site-688 | docusaurus.io | [https://docusaurus.io/robots.txt](https://docusaurus.io/robots.txt) | 523 ms | 206 ms | 211 ms |
| site-689 | www.11ty.dev | [https://www.11ty.dev/robots.txt](https://www.11ty.dev/robots.txt) | 157 ms | 228 ms | 239 ms |
| site-690 | quarto.org | [https://quarto.org/robots.txt](https://quarto.org/robots.txt) | 147 ms | 190 ms | 207 ms |
| site-691 | jekyllrb.com | [https://jekyllrb.com/robots.txt](https://jekyllrb.com/robots.txt) | 141 ms | 196 ms | 194 ms |
| site-692 | pandoc.org | [https://pandoc.org/robots.txt](https://pandoc.org/robots.txt) | 154 ms | 202 ms | 226 ms |
| site-693 | typst.app | [https://typst.app/robots.txt](https://typst.app/robots.txt) | 363 ms | 656 ms | 524 ms |
| site-694 | getpelican.com | [https://getpelican.com/robots.txt](https://getpelican.com/robots.txt) | 302 ms | 323 ms | 339 ms |
| site-695 | www.mathjax.org | [https://www.mathjax.org/robots.txt](https://www.mathjax.org/robots.txt) | 193 ms | 537 ms | 215 ms |
| site-696 | numpy.org | [https://numpy.org/robots.txt](https://numpy.org/robots.txt) | 164 ms | 212 ms | 215 ms |
| site-697 | www.latex-project.org | [https://www.latex-project.org/robots.txt](https://www.latex-project.org/robots.txt) | 698 ms | 364 ms | 353 ms |
| site-698 | curl.se | [https://curl.se/robots.txt](https://curl.se/robots.txt) | 157 ms | 220 ms | 343 ms |
| site-699 | openssl-library.org | [https://openssl-library.org/robots.txt](https://openssl-library.org/robots.txt) | 202 ms | 575 ms | 252 ms |
| site-700 | openvpn.net | [https://openvpn.net/robots.txt](https://openvpn.net/robots.txt) | 168 ms | 491 ms | 209 ms |
| site-701 | nmap.org | [https://nmap.org/robots.txt](https://nmap.org/robots.txt) | 206 ms | 217 ms | 204 ms |
| site-702 | www.wireshark.org | [https://www.wireshark.org/robots.txt](https://www.wireshark.org/robots.txt) | 179 ms | 213 ms | 208 ms |
| site-703 | www.envoyproxy.io | [https://www.envoyproxy.io/robots.txt](https://www.envoyproxy.io/robots.txt) | 147 ms | 219 ms | 214 ms |
| site-704 | www.strongswan.org | [https://www.strongswan.org/robots.txt](https://www.strongswan.org/robots.txt) | 297 ms | 334 ms | 350 ms |
| site-705 | traefik.io | [https://traefik.io/robots.txt](https://traefik.io/robots.txt) | 551 ms | 561 ms | 238 ms |
| site-706 | www.squid-cache.org | [https://www.squid-cache.org/robots.txt](https://www.squid-cache.org/robots.txt) | 186 ms | 819 ms | 243 ms |
| site-707 | www.powerdns.com | [https://www.powerdns.com/robots.txt](https://www.powerdns.com/robots.txt) | 724 ms | 224 ms | 208 ms |
| site-708 | www.isc.org | [https://www.isc.org/robots.txt](https://www.isc.org/robots.txt) | 366 ms | 262 ms | 192 ms |
| site-710 | www.nlnetlabs.nl | [https://www.nlnetlabs.nl/robots.txt](https://www.nlnetlabs.nl/robots.txt) | 285 ms | 343 ms | 347 ms |
| site-711 | bunny.net | [https://bunny.net/robots.txt](https://bunny.net/robots.txt) | 148 ms | 193 ms | 198 ms |
| site-712 | www.princeton.edu | [https://www.princeton.edu/robots.txt](https://www.princeton.edu/robots.txt) | 162 ms | 505 ms | 206 ms |
| site-713 | crates.io | [https://crates.io/robots.txt](https://crates.io/robots.txt) | 141 ms | 517 ms | 190 ms |
| site-714 | cloudinary.com | [https://cloudinary.com/robots.txt](https://cloudinary.com/robots.txt) | 164 ms | 525 ms | 210 ms |
| site-715 | rubygems.org | [https://rubygems.org/robots.txt](https://rubygems.org/robots.txt) | 136 ms | 206 ms | 194 ms |
| site-716 | bun.sh | [https://bun.sh/robots.txt](https://bun.sh/robots.txt) | 178 ms | 540 ms | 661 ms |
| site-717 | deno.com | [https://deno.com/robots.txt](https://deno.com/robots.txt) | 181 ms | 247 ms | 247 ms |
| site-718 | www.upenn.edu | [https://www.upenn.edu/robots.txt](https://www.upenn.edu/robots.txt) | 145 ms | 485 ms | 197 ms |
| site-719 | packagist.org | [https://packagist.org/robots.txt](https://packagist.org/robots.txt) | 144 ms | 499 ms | 192 ms |
| site-720 | mariadb.org | [https://mariadb.org/robots.txt](https://mariadb.org/robots.txt) | 289 ms | 337 ms | 355 ms |
| site-721 | www.brown.edu | [https://www.brown.edu/robots.txt](https://www.brown.edu/robots.txt) | 512 ms | 198 ms | 208 ms |
| site-722 | duckdb.org | [https://duckdb.org/robots.txt](https://duckdb.org/robots.txt) | 205 ms | 236 ms | 234 ms |
| site-723 | imagekit.io | [https://imagekit.io/robots.txt](https://imagekit.io/robots.txt) | 447 ms | 191 ms | 220 ms |
| site-724 | clickhouse.com | [https://clickhouse.com/robots.txt](https://clickhouse.com/robots.txt) | 798 ms | 489 ms | 310 ms |
| site-725 | roc-lang.org | [https://roc-lang.org/robots.txt](https://roc-lang.org/robots.txt) | 179 ms | 213 ms | 224 ms |
| site-726 | uploadcare.com | [https://uploadcare.com/robots.txt](https://uploadcare.com/robots.txt) | 331 ms | 281 ms | 315 ms |
| site-727 | hex.pm | [https://hex.pm/robots.txt](https://hex.pm/robots.txt) | 202 ms | 248 ms | 255 ms |
| site-728 | www.dartmouth.edu | [https://www.dartmouth.edu/robots.txt](https://www.dartmouth.edu/robots.txt) | 206 ms | 252 ms | 530 ms |
| site-729 | vlang.io | [https://vlang.io/robots.txt](https://vlang.io/robots.txt) | 521 ms | 206 ms | 208 ms |
| site-730 | nim-lang.org | [https://nim-lang.org/robots.txt](https://nim-lang.org/robots.txt) | 157 ms | 203 ms | 203 ms |
| site-731 | www.jhu.edu | [https://www.jhu.edu/robots.txt](https://www.jhu.edu/robots.txt) | 148 ms | 200 ms | 206 ms |
| site-733 | pub.dev | [https://pub.dev/robots.txt](https://pub.dev/robots.txt) | 198 ms | 560 ms | 249 ms |
| site-734 | wasabi.com | [https://wasabi.com/robots.txt](https://wasabi.com/robots.txt) | 155 ms | 214 ms | 210 ms |
| site-735 | central.sonatype.com | [https://central.sonatype.com/robots.txt](https://central.sonatype.com/robots.txt) | 176 ms | 952 ms | 654 ms |
| site-736 | racket-lang.org | [https://racket-lang.org/robots.txt](https://racket-lang.org/robots.txt) | 228 ms | 270 ms | 277 ms |
| site-737 | vitess.io | [https://vitess.io/robots.txt](https://vitess.io/robots.txt) | 142 ms | 480 ms | 210 ms |
| site-738 | clojure.org | [https://clojure.org/robots.txt](https://clojure.org/robots.txt) | 142 ms | 186 ms | 398 ms |
| site-739 | tidbcloud.com | [https://tidbcloud.com/robots.txt](https://tidbcloud.com/robots.txt) | 143 ms | 196 ms | 197 ms |
| site-740 | www.uchicago.edu | [https://www.uchicago.edu/robots.txt](https://www.uchicago.edu/robots.txt) | 579 ms | 528 ms | 239 ms |
| site-741 | www.rice.edu | [https://www.rice.edu/robots.txt](https://www.rice.edu/robots.txt) | 135 ms | 186 ms | 189 ms |
| site-742 | gradle.org | [https://gradle.org/robots.txt](https://gradle.org/robots.txt) | 149 ms | 197 ms | 196 ms |
| site-743 | supabase.com | [https://supabase.com/robots.txt](https://supabase.com/robots.txt) | 173 ms | 280 ms | 233 ms |
| site-744 | www.northwestern.edu | [https://www.northwestern.edu/robots.txt](https://www.northwestern.edu/robots.txt) | 570 ms | 238 ms | 240 ms |
| site-745 | ceph.io | [https://ceph.io/robots.txt](https://ceph.io/robots.txt) | 199 ms | 273 ms | 259 ms |
| site-746 | planetscale.com | [https://planetscale.com/robots.txt](https://planetscale.com/robots.txt) | 173 ms | 584 ms | 250 ms |
| site-747 | leiningen.org | [https://leiningen.org/robots.txt](https://leiningen.org/robots.txt) | 164 ms | 209 ms | 227 ms |
| site-748 | dlang.org | [https://dlang.org/robots.txt](https://dlang.org/robots.txt) | 157 ms | 507 ms | 216 ms |
| site-749 | www.nd.edu | [https://www.nd.edu/robots.txt](https://www.nd.edu/robots.txt) | 199 ms | 246 ms | 266 ms |
| site-750 | turso.tech | [https://turso.tech/robots.txt](https://turso.tech/robots.txt) | 173 ms | 797 ms | 222 ms |
| site-751 | www.emory.edu | [https://www.emory.edu/robots.txt](https://www.emory.edu/robots.txt) | 548 ms | 249 ms | 234 ms |
| site-752 | www.georgetown.edu | [https://www.georgetown.edu/robots.txt](https://www.georgetown.edu/robots.txt) | 141 ms | 194 ms | 197 ms |
| site-753 | pdm-project.org | [https://pdm-project.org/robots.txt](https://pdm-project.org/robots.txt) | 349 ms | 821 ms | 223 ms |
| site-754 | www.tufts.edu | [https://www.tufts.edu/robots.txt](https://www.tufts.edu/robots.txt) | 144 ms | 192 ms | 192 ms |
| site-755 | fsharp.org | [https://fsharp.org/robots.txt](https://fsharp.org/robots.txt) | 194 ms | 198 ms | 193 ms |
| site-756 | www.bu.edu | [https://www.bu.edu/robots.txt](https://www.bu.edu/robots.txt) | 147 ms | 185 ms | 211 ms |
| site-757 | restic.net | [https://restic.net/robots.txt](https://restic.net/robots.txt) | 277 ms | 615 ms | 347 ms |
| site-758 | tox.wiki | [https://tox.wiki/robots.txt](https://tox.wiki/robots.txt) | 168 ms | 202 ms | 207 ms |
| site-759 | rescript-lang.org | [https://rescript-lang.org/robots.txt](https://rescript-lang.org/robots.txt) | 219 ms | 552 ms | 337 ms |
| site-760 | redis.io | [https://redis.io/robots.txt](https://redis.io/robots.txt) | 322 ms | 366 ms | 372 ms |
| site-761 | duplicati.com | [https://duplicati.com/robots.txt](https://duplicati.com/robots.txt) | 163 ms | 519 ms | 215 ms |
| site-762 | nox.thea.codes | [https://nox.thea.codes/robots.txt](https://nox.thea.codes/robots.txt) | 166 ms | 210 ms | 643 ms |
| site-763 | kopia.io | [https://kopia.io/robots.txt](https://kopia.io/robots.txt) | 171 ms | 220 ms | 218 ms |
| site-764 | valkey.io | [https://valkey.io/robots.txt](https://valkey.io/robots.txt) | 488 ms | 486 ms | 191 ms |
| site-765 | elm-lang.org | [https://elm-lang.org/robots.txt](https://elm-lang.org/robots.txt) | 163 ms | 224 ms | 223 ms |
| site-766 | www.rochester.edu | [https://www.rochester.edu/robots.txt](https://www.rochester.edu/robots.txt) | 199 ms | 536 ms | 267 ms |
| site-767 | haxe.org | [https://haxe.org/robots.txt](https://haxe.org/robots.txt) | 520 ms | 205 ms | 219 ms |
| site-768 | www.usc.edu | [https://www.usc.edu/robots.txt](https://www.usc.edu/robots.txt) | 143 ms | 190 ms | 197 ms |
| site-769 | www.ucla.edu | [https://www.ucla.edu/robots.txt](https://www.ucla.edu/robots.txt) | 140 ms | 188 ms | 195 ms |
| site-770 | pnpm.io | [https://pnpm.io/robots.txt](https://pnpm.io/robots.txt) | 157 ms | 201 ms | 233 ms |
| site-771 | memcached.org | [https://memcached.org/robots.txt](https://memcached.org/robots.txt) | 149 ms | 477 ms | 201 ms |
| site-772 | cassandra.apache.org | [https://cassandra.apache.org/robots.txt](https://cassandra.apache.org/robots.txt) | 136 ms | 186 ms | 188 ms |
| site-773 | www.ucsb.edu | [https://www.ucsb.edu/robots.txt](https://www.ucsb.edu/robots.txt) | 139 ms | 195 ms | 210 ms |
| site-774 | www.ucsd.edu | [https://www.ucsd.edu/robots.txt](https://www.ucsd.edu/robots.txt) | 165 ms | 217 ms | 222 ms |
| site-775 | upcloud.com | [https://upcloud.com/robots.txt](https://upcloud.com/robots.txt) | 154 ms | 515 ms | 544 ms |
| site-776 | ballerina.io | [https://ballerina.io/robots.txt](https://ballerina.io/robots.txt) | 139 ms | 184 ms | 212 ms |
| site-777 | kamatera.com | [https://kamatera.com/robots.txt](https://kamatera.com/robots.txt) | 522 ms | 240 ms | 214 ms |
| site-778 | www.ucdavis.edu | [https://www.ucdavis.edu/robots.txt](https://www.ucdavis.edu/robots.txt) | 142 ms | 195 ms | 190 ms |
| site-779 | nx.dev | [https://nx.dev/robots.txt](https://nx.dev/robots.txt) | 287 ms | 358 ms | 356 ms |
| site-780 | phoenixnap.com | [https://phoenixnap.com/robots.txt](https://phoenixnap.com/robots.txt) | 181 ms | 506 ms | 217 ms |
| site-781 | www.ucr.edu | [https://www.ucr.edu/robots.txt](https://www.ucr.edu/robots.txt) | 144 ms | 481 ms | 195 ms |
| site-782 | fennel-lang.org | [https://fennel-lang.org/robots.txt](https://fennel-lang.org/robots.txt) | 526 ms | 214 ms | 227 ms |
| site-783 | victoriametrics.com | [https://victoriametrics.com/robots.txt](https://victoriametrics.com/robots.txt) | 136 ms | 186 ms | 190 ms |
| site-784 | cherryservers.com | [https://cherryservers.com/robots.txt](https://cherryservers.com/robots.txt) | 230 ms | 553 ms | 247 ms |
| site-785 | gruntjs.com | [https://gruntjs.com/robots.txt](https://gruntjs.com/robots.txt) | 376 ms | 218 ms | 213 ms |
| site-786 | greptime.com | [https://greptime.com/robots.txt](https://greptime.com/robots.txt) | 397 ms | 239 ms | 698 ms |
| site-787 | contabo.com | [https://contabo.com/robots.txt](https://contabo.com/robots.txt) | 187 ms | 230 ms | 212 ms |
| site-788 | www.washington.edu | [https://www.washington.edu/robots.txt](https://www.washington.edu/robots.txt) | 565 ms | 518 ms | 221 ms |
| site-789 | factorcode.org | [https://factorcode.org/robots.txt](https://factorcode.org/robots.txt) | 411 ms | 535 ms | 259 ms |
| site-790 | www.wisc.edu | [https://www.wisc.edu/robots.txt](https://www.wisc.edu/robots.txt) | 171 ms | 830 ms | 220 ms |
| site-791 | vyperlang.org | [https://vyperlang.org/robots.txt](https://vyperlang.org/robots.txt) | 207 ms | 244 ms | 255 ms |
| site-792 | racknerd.com | [https://racknerd.com/robots.txt](https://racknerd.com/robots.txt) | 163 ms | 204 ms | 211 ms |
| site-793 | typesense.org | [https://typesense.org/robots.txt](https://typesense.org/robots.txt) | 145 ms | 185 ms | 191 ms |
| site-794 | biomejs.dev | [https://biomejs.dev/robots.txt](https://biomejs.dev/robots.txt) | 144 ms | 194 ms | 206 ms |
| site-795 | hosthatch.com | [https://hosthatch.com/robots.txt](https://hosthatch.com/robots.txt) | 553 ms | 228 ms | 237 ms |
| site-796 | www.purdue.edu | [https://www.purdue.edu/robots.txt](https://www.purdue.edu/robots.txt) | 558 ms | 242 ms | 244 ms |
| site-797 | agda.readthedocs.io | [https://agda.readthedocs.io/robots.txt](https://agda.readthedocs.io/robots.txt) | 526 ms | 202 ms | 213 ms |
| site-798 | www.gatech.edu | [https://www.gatech.edu/robots.txt](https://www.gatech.edu/robots.txt) | 135 ms | 486 ms | 190 ms |
| site-799 | ramnode.com | [https://ramnode.com/robots.txt](https://ramnode.com/robots.txt) | 161 ms | 209 ms | 213 ms |
| site-800 | vitest.dev | [https://vitest.dev/robots.txt](https://vitest.dev/robots.txt) | 146 ms | 210 ms | 202 ms |
| site-801 | www.utexas.edu | [https://www.utexas.edu/robots.txt](https://www.utexas.edu/robots.txt) | 145 ms | 190 ms | 197 ms |
| site-802 | vespa.ai | [https://vespa.ai/robots.txt](https://vespa.ai/robots.txt) | 430 ms | 278 ms | 273 ms |
| site-803 | www.tamu.edu | [https://www.tamu.edu/robots.txt](https://www.tamu.edu/robots.txt) | 525 ms | 190 ms | 195 ms |
| site-804 | milvus.io | [https://milvus.io/robots.txt](https://milvus.io/robots.txt) | 163 ms | 524 ms | 222 ms |
| site-805 | qdrant.tech | [https://qdrant.tech/robots.txt](https://qdrant.tech/robots.txt) | 148 ms | 490 ms | 199 ms |
| site-806 | weaviate.io | [https://weaviate.io/robots.txt](https://weaviate.io/robots.txt) | 146 ms | 193 ms | 196 ms |
| site-807 | www.virginia.edu | [https://www.virginia.edu/robots.txt](https://www.virginia.edu/robots.txt) | 187 ms | 484 ms | 204 ms |
| site-808 | testing-library.com | [https://testing-library.com/robots.txt](https://testing-library.com/robots.txt) | 174 ms | 216 ms | 248 ms |
| site-809 | playwright.dev | [https://playwright.dev/robots.txt](https://playwright.dev/robots.txt) | 138 ms | 184 ms | 191 ms |
| site-810 | aplwiki.com | [https://aplwiki.com/robots.txt](https://aplwiki.com/robots.txt) | 533 ms | 941 ms | 603 ms |
| site-811 | www.wm.edu | [https://www.wm.edu/robots.txt](https://www.wm.edu/robots.txt) | 140 ms | 188 ms | 190 ms |
| site-812 | gcore.com | [https://gcore.com/robots.txt](https://gcore.com/robots.txt) | 178 ms | 218 ms | 204 ms |
| site-813 | www.psu.edu | [https://www.psu.edu/robots.txt](https://www.psu.edu/robots.txt) | 287 ms | 189 ms | 187 ms |
| site-814 | www.osu.edu | [https://www.osu.edu/robots.txt](https://www.osu.edu/robots.txt) | 773 ms | 854 ms | 267 ms |
| site-815 | faiss.ai | [https://faiss.ai/robots.txt](https://faiss.ai/robots.txt) | 165 ms | 477 ms | 207 ms |
| site-816 | neo4j.com | [https://neo4j.com/robots.txt](https://neo4j.com/robots.txt) | 236 ms | 509 ms | 245 ms |
| site-817 | webdriver.io | [https://webdriver.io/robots.txt](https://webdriver.io/robots.txt) | 164 ms | 206 ms | 204 ms |
| site-819 | www.fsu.edu | [https://www.fsu.edu/robots.txt](https://www.fsu.edu/robots.txt) | 156 ms | 783 ms | 429 ms |
| site-820 | www.ufl.edu | [https://www.ufl.edu/robots.txt](https://www.ufl.edu/robots.txt) | 192 ms | 548 ms | 245 ms |
| site-821 | memgraph.com | [https://memgraph.com/robots.txt](https://memgraph.com/robots.txt) | 170 ms | 222 ms | 258 ms |
| site-822 | robotframework.org | [https://robotframework.org/robots.txt](https://robotframework.org/robots.txt) | 138 ms | 185 ms | 208 ms |
| site-823 | www.asu.edu | [https://www.asu.edu/robots.txt](https://www.asu.edu/robots.txt) | 519 ms | 199 ms | 205 ms |
| site-824 | surrealdb.com | [https://surrealdb.com/robots.txt](https://surrealdb.com/robots.txt) | 307 ms | 385 ms | 224 ms |
| site-825 | www.arizona.edu | [https://www.arizona.edu/robots.txt](https://www.arizona.edu/robots.txt) | 143 ms | 494 ms | 189 ms |
| site-826 | cmake.org | [https://cmake.org/robots.txt](https://cmake.org/robots.txt) | 200 ms | 251 ms | 261 ms |
| site-827 | gauge.org | [https://gauge.org/robots.txt](https://gauge.org/robots.txt) | 144 ms | 487 ms | 205 ms |
| site-828 | www.colorado.edu | [https://www.colorado.edu/robots.txt](https://www.colorado.edu/robots.txt) | 137 ms | 189 ms | 191 ms |
| site-829 | railway.com | [https://railway.com/robots.txt](https://railway.com/robots.txt) | 135 ms | 183 ms | 187 ms |
| site-830 | www.utah.edu | [https://www.utah.edu/robots.txt](https://www.utah.edu/robots.txt) | 152 ms | 502 ms | 218 ms |
| site-831 | ninja-build.org | [https://ninja-build.org/robots.txt](https://ninja-build.org/robots.txt) | 159 ms | 499 ms | 209 ms |
| site-832 | locust.io | [https://locust.io/robots.txt](https://locust.io/robots.txt) | 578 ms | 236 ms | 227 ms |
| site-833 | www.iastate.edu | [https://www.iastate.edu/robots.txt](https://www.iastate.edu/robots.txt) | 203 ms | 240 ms | 262 ms |
| site-834 | bazel.build | [https://bazel.build/robots.txt](https://bazel.build/robots.txt) | 538 ms | 558 ms | 237 ms |
| site-835 | northflank.com | [https://northflank.com/robots.txt](https://northflank.com/robots.txt) | 516 ms | 186 ms | 194 ms |
| site-836 | gcc.gnu.org | [https://gcc.gnu.org/robots.txt](https://gcc.gnu.org/robots.txt) | 272 ms | 584 ms | 787 ms |
| site-837 | zeabur.com | [https://zeabur.com/robots.txt](https://zeabur.com/robots.txt) | 146 ms | 495 ms | 206 ms |
| site-838 | gatling.io | [https://gatling.io/robots.txt](https://gatling.io/robots.txt) | 159 ms | 207 ms | 216 ms |
| site-840 | dbdiagram.io | [https://dbdiagram.io/robots.txt](https://dbdiagram.io/robots.txt) | 165 ms | 512 ms | 207 ms |
| site-842 | www.rutgers.edu | [https://www.rutgers.edu/robots.txt](https://www.rutgers.edu/robots.txt) | 494 ms | 548 ms | 264 ms |
| site-843 | wiremock.org | [https://wiremock.org/robots.txt](https://wiremock.org/robots.txt) | 202 ms | 227 ms | 234 ms |
| site-844 | auth0.com | [https://auth0.com/robots.txt](https://auth0.com/robots.txt) | 399 ms | 427 ms | 638 ms |
| site-845 | mock-server.com | [https://mock-server.com/robots.txt](https://mock-server.com/robots.txt) | 299 ms | 344 ms | 342 ms |
| site-846 | dbdocs.io | [https://dbdocs.io/robots.txt](https://dbdocs.io/robots.txt) | 150 ms | 202 ms | 433 ms |
| site-847 | hoppscotch.io | [https://hoppscotch.io/robots.txt](https://hoppscotch.io/robots.txt) | 548 ms | 200 ms | 217 ms |
| site-848 | clerk.com | [https://clerk.com/robots.txt](https://clerk.com/robots.txt) | 214 ms | 217 ms | 216 ms |
| site-849 | bell-sw.com | [https://bell-sw.com/robots.txt](https://bell-sw.com/robots.txt) | 156 ms | 490 ms | 194 ms |
| site-850 | stytch.com | [https://stytch.com/robots.txt](https://stytch.com/robots.txt) | 393 ms | 636 ms | 281 ms |
| site-851 | httpie.io | [https://httpie.io/robots.txt](https://httpie.io/robots.txt) | 202 ms | 239 ms | 243 ms |
| site-852 | www.ucl.ac.uk | [https://www.ucl.ac.uk/robots.txt](https://www.ucl.ac.uk/robots.txt) | 398 ms | 260 ms | 285 ms |
| site-853 | adoptium.net | [https://adoptium.net/robots.txt](https://adoptium.net/robots.txt) | 551 ms | 184 ms | 202 ms |
| site-854 | workos.com | [https://workos.com/robots.txt](https://workos.com/robots.txt) | 162 ms | 195 ms | 209 ms |
| site-855 | azul.com | [https://azul.com/robots.txt](https://azul.com/robots.txt) | 152 ms | 491 ms | 228 ms |
| site-857 | zitadel.com | [https://zitadel.com/robots.txt](https://zitadel.com/robots.txt) | 421 ms | 690 ms | 206 ms |
| site-858 | atlasgo.io | [https://atlasgo.io/robots.txt](https://atlasgo.io/robots.txt) | 172 ms | 241 ms | 198 ms |
| site-859 | swagger.io | [https://swagger.io/robots.txt](https://swagger.io/robots.txt) | 197 ms | 517 ms | 242 ms |
| site-861 | goose.readthedocs.io | [https://goose.readthedocs.io/robots.txt](https://goose.readthedocs.io/robots.txt) | 156 ms | 219 ms | 212 ms |
| site-862 | www.manchester.ac.uk | [https://www.manchester.ac.uk/robots.txt](https://www.manchester.ac.uk/robots.txt) | 302 ms | 622 ms | 341 ms |
| site-863 | kafka.apache.org | [https://kafka.apache.org/robots.txt](https://kafka.apache.org/robots.txt) | 534 ms | 186 ms | 398 ms |
| site-864 | visualstudio.microsoft.com | [https://visualstudio.microsoft.com/robots.txt](https://visualstudio.microsoft.com/robots.txt) | 201 ms | 192 ms | 195 ms |
| site-865 | www.bristol.ac.uk | [https://www.bristol.ac.uk/robots.txt](https://www.bristol.ac.uk/robots.txt) | 289 ms | 343 ms | 331 ms |
| site-866 | vscodium.com | [https://vscodium.com/robots.txt](https://vscodium.com/robots.txt) | 154 ms | 202 ms | 208 ms |
| site-867 | pulsar.apache.org | [https://pulsar.apache.org/robots.txt](https://pulsar.apache.org/robots.txt) | 155 ms | 501 ms | 188 ms |
| site-868 | neovim.io | [https://neovim.io/robots.txt](https://neovim.io/robots.txt) | 155 ms | 226 ms | 210 ms |
| site-869 | www.st-andrews.ac.uk | [https://www.st-andrews.ac.uk/robots.txt](https://www.st-andrews.ac.uk/robots.txt) | 294 ms | 623 ms | 342 ms |
| site-871 | fusionauth.io | [https://fusionauth.io/robots.txt](https://fusionauth.io/robots.txt) | 145 ms | 189 ms | 194 ms |
| site-872 | the-guild.dev | [https://the-guild.dev/robots.txt](https://the-guild.dev/robots.txt) | 566 ms | 235 ms | 235 ms |
| site-873 | nats.io | [https://nats.io/robots.txt](https://nats.io/robots.txt) | 530 ms | 227 ms | 199 ms |
| site-874 | helix-editor.com | [https://helix-editor.com/robots.txt](https://helix-editor.com/robots.txt) | 138 ms | 187 ms | 189 ms |
| site-875 | hasura.io | [https://hasura.io/robots.txt](https://hasura.io/robots.txt) | 171 ms | 483 ms | 213 ms |
| site-876 | zed.dev | [https://zed.dev/robots.txt](https://zed.dev/robots.txt) | 162 ms | 210 ms | 218 ms |
| site-877 | supertokens.com | [https://supertokens.com/robots.txt](https://supertokens.com/robots.txt) | 205 ms | 236 ms | 243 ms |
| site-878 | logto.io | [https://logto.io/robots.txt](https://logto.io/robots.txt) | 180 ms | 206 ms | 219 ms |
| site-880 | geany.org | [https://geany.org/robots.txt](https://geany.org/robots.txt) | 288 ms | 641 ms | 367 ms |
| site-881 | typeorm.io | [https://typeorm.io/robots.txt](https://typeorm.io/robots.txt) | 505 ms | 229 ms | 278 ms |
| site-882 | oauth.net | [https://oauth.net/robots.txt](https://oauth.net/robots.txt) | 161 ms | 204 ms | 213 ms |
| site-883 | kate-editor.org | [https://kate-editor.org/robots.txt](https://kate-editor.org/robots.txt) | 289 ms | 323 ms | 343 ms |
| site-884 | www.southampton.ac.uk | [https://www.southampton.ac.uk/robots.txt](https://www.southampton.ac.uk/robots.txt) | 159 ms | 200 ms | 215 ms |
| site-885 | sequelize.org | [https://sequelize.org/robots.txt](https://sequelize.org/robots.txt) | 169 ms | 217 ms | 228 ms |
| site-886 | openid.net | [https://openid.net/robots.txt](https://openid.net/robots.txt) | 136 ms | 187 ms | 200 ms |
| site-887 | vernemq.com | [https://vernemq.com/robots.txt](https://vernemq.com/robots.txt) | 156 ms | 211 ms | 205 ms |
| site-888 | www.york.ac.uk | [https://www.york.ac.uk/robots.txt](https://www.york.ac.uk/robots.txt) | 150 ms | 559 ms | 195 ms |
| site-889 | knexjs.org | [https://knexjs.org/robots.txt](https://knexjs.org/robots.txt) | 170 ms | 203 ms | 215 ms |
| site-890 | passkeys.dev | [https://passkeys.dev/robots.txt](https://passkeys.dev/robots.txt) | 414 ms | 501 ms | 218 ms |
| site-891 | www.lancaster.ac.uk | [https://www.lancaster.ac.uk/robots.txt](https://www.lancaster.ac.uk/robots.txt) | 277 ms | 614 ms | 336 ms |
| site-892 | textmate.org | [https://textmate.org/robots.txt](https://textmate.org/robots.txt) | 267 ms | 590 ms | 323 ms |
| site-893 | www.exeter.ac.uk | [https://www.exeter.ac.uk/robots.txt](https://www.exeter.ac.uk/robots.txt) | 145 ms | 479 ms | 192 ms |
| site-894 | temporal.io | [https://temporal.io/robots.txt](https://temporal.io/robots.txt) | 186 ms | 244 ms | 206 ms |
| site-895 | emacsredux.com | [https://emacsredux.com/robots.txt](https://emacsredux.com/robots.txt) | 134 ms | 183 ms | 189 ms |
| site-896 | bun.uptrace.dev | [https://bun.uptrace.dev/robots.txt](https://bun.uptrace.dev/robots.txt) | 134 ms | 187 ms | 190 ms |
| site-897 | airbyte.com | [https://airbyte.com/robots.txt](https://airbyte.com/robots.txt) | 152 ms | 201 ms | 206 ms |
| site-898 | www.bath.ac.uk | [https://www.bath.ac.uk/robots.txt](https://www.bath.ac.uk/robots.txt) | 635 ms | 344 ms | 358 ms |
| site-899 | meltano.com | [https://meltano.com/robots.txt](https://meltano.com/robots.txt) | 148 ms | 191 ms | 202 ms |
| site-900 | solokeys.com | [https://solokeys.com/robots.txt](https://solokeys.com/robots.txt) | 320 ms | 588 ms | 330 ms |
| site-901 | dagster.io | [https://dagster.io/robots.txt](https://dagster.io/robots.txt) | 530 ms | 532 ms | 274 ms |
| site-902 | www.liverpool.ac.uk | [https://www.liverpool.ac.uk/robots.txt](https://www.liverpool.ac.uk/robots.txt) | 277 ms | 338 ms | 352 ms |
| site-903 | www.unsw.edu.au | [https://www.unsw.edu.au/robots.txt](https://www.unsw.edu.au/robots.txt) | 650 ms | 424 ms | 243 ms |
| site-904 | kestra.io | [https://kestra.io/robots.txt](https://kestra.io/robots.txt) | 155 ms | 483 ms | 398 ms |
| site-905 | openbao.org | [https://openbao.org/robots.txt](https://openbao.org/robots.txt) | 135 ms | 788 ms | 187 ms |
| site-906 | martinfowler.com | [https://martinfowler.com/robots.txt](https://martinfowler.com/robots.txt) | 196 ms | 246 ms | 259 ms |
| site-907 | infisical.com | [https://infisical.com/robots.txt](https://infisical.com/robots.txt) | 868 ms | 567 ms | 247 ms |
| site-908 | refactoring.com | [https://refactoring.com/robots.txt](https://refactoring.com/robots.txt) | 195 ms | 537 ms | 252 ms |
| site-909 | n8n.io | [https://n8n.io/robots.txt](https://n8n.io/robots.txt) | 157 ms | 528 ms | 212 ms |
| site-910 | web.dev | [https://web.dev/robots.txt](https://web.dev/robots.txt) | 448 ms | 364 ms | 586 ms |
| site-911 | spark.apache.org | [https://spark.apache.org/robots.txt](https://spark.apache.org/robots.txt) | 180 ms | 483 ms | 190 ms |
| site-912 | www.anu.edu.au | [https://www.anu.edu.au/robots.txt](https://www.anu.edu.au/robots.txt) | 270 ms | 632 ms | 345 ms |
| site-913 | developer.chrome.com | [https://developer.chrome.com/robots.txt](https://developer.chrome.com/robots.txt) | 391 ms | 352 ms | 339 ms |
| site-914 | developer.mozilla.org | [https://developer.mozilla.org/robots.txt](https://developer.mozilla.org/robots.txt) | 135 ms | 193 ms | 197 ms |
| site-915 | beam.apache.org | [https://beam.apache.org/robots.txt](https://beam.apache.org/robots.txt) | 293 ms | 194 ms | 194 ms |
| site-916 | airflow.apache.org | [https://airflow.apache.org/robots.txt](https://airflow.apache.org/robots.txt) | 131 ms | 189 ms | 188 ms |
| site-917 | www.uwa.edu.au | [https://www.uwa.edu.au/robots.txt](https://www.uwa.edu.au/robots.txt) | 254 ms | 572 ms | 301 ms |
| site-918 | psono.com | [https://psono.com/robots.txt](https://psono.com/robots.txt) | 210 ms | 215 ms | 234 ms |
| site-919 | www.wgtn.ac.nz | [https://www.wgtn.ac.nz/robots.txt](https://www.wgtn.ac.nz/robots.txt) | 133 ms | 188 ms | 190 ms |
| site-920 | kubernetes.io | [https://kubernetes.io/robots.txt](https://kubernetes.io/robots.txt) | 146 ms | 483 ms | 196 ms |
| site-921 | hudi.apache.org | [https://hudi.apache.org/robots.txt](https://hudi.apache.org/robots.txt) | 194 ms | 610 ms | 193 ms |
| site-922 | delta.io | [https://delta.io/robots.txt](https://delta.io/robots.txt) | 169 ms | 373 ms | 199 ms |
| site-923 | www.massey.ac.nz | [https://www.massey.ac.nz/robots.txt](https://www.massey.ac.nz/robots.txt) | 207 ms | 302 ms | 204 ms |
| site-924 | www.waikato.ac.nz | [https://www.waikato.ac.nz/robots.txt](https://www.waikato.ac.nz/robots.txt) | 439 ms | 471 ms | 532 ms |
| site-925 | html.spec.whatwg.org | [https://html.spec.whatwg.org/robots.txt](https://html.spec.whatwg.org/robots.txt) | 286 ms | 259 ms | 382 ms |
| site-926 | whatwg.org | [https://whatwg.org/robots.txt](https://whatwg.org/robots.txt) | 211 ms | 620 ms | 551 ms |
| site-927 | trino.io | [https://trino.io/robots.txt](https://trino.io/robots.txt) | 172 ms | 820 ms | 214 ms |
| site-928 | www.ntu.edu.sg | [https://www.ntu.edu.sg/robots.txt](https://www.ntu.edu.sg/robots.txt) | 1036 ms | 938 ms | 539 ms |
| site-929 | v8.dev | [https://v8.dev/robots.txt](https://v8.dev/robots.txt) | 135 ms | 187 ms | 192 ms |
| site-930 | druid.apache.org | [https://druid.apache.org/robots.txt](https://druid.apache.org/robots.txt) | 132 ms | 199 ms | 190 ms |
| site-931 | portainer.io | [https://portainer.io/robots.txt](https://portainer.io/robots.txt) | 203 ms | 563 ms | 239 ms |
| site-932 | servo.org | [https://servo.org/robots.txt](https://servo.org/robots.txt) | 159 ms | 524 ms | 213 ms |
| site-933 | www.sutd.edu.sg | [https://www.sutd.edu.sg/robots.txt](https://www.sutd.edu.sg/robots.txt) | 852 ms | 608 ms | 652 ms |
| site-934 | pinot.apache.org | [https://pinot.apache.org/robots.txt](https://pinot.apache.org/robots.txt) | 502 ms | 488 ms | 193 ms |
| site-935 | mdn.dev | [https://mdn.dev/robots.txt](https://mdn.dev/robots.txt) | 136 ms | 192 ms | 192 ms |
| site-936 | arrow.apache.org | [https://arrow.apache.org/robots.txt](https://arrow.apache.org/robots.txt) | 289 ms | 490 ms | 209 ms |
| site-937 | caprover.com | [https://caprover.com/robots.txt](https://caprover.com/robots.txt) | 512 ms | 775 ms | 192 ms |
| site-938 | parquet.apache.org | [https://parquet.apache.org/robots.txt](https://parquet.apache.org/robots.txt) | 137 ms | 627 ms | 199 ms |
| site-939 | coolify.io | [https://coolify.io/robots.txt](https://coolify.io/robots.txt) | 688 ms | 362 ms | 359 ms |
| site-940 | exercism.org | [https://exercism.org/robots.txt](https://exercism.org/robots.txt) | 180 ms | 614 ms | 248 ms |
| site-941 | www.kyoto-u.ac.jp | [https://www.kyoto-u.ac.jp/robots.txt](https://www.kyoto-u.ac.jp/robots.txt) | 497 ms | 499 ms | 187 ms |
| site-942 | datafusion.apache.org | [https://datafusion.apache.org/robots.txt](https://datafusion.apache.org/robots.txt) | 348 ms | 515 ms | 195 ms |
| site-943 | leetcode.com | [https://leetcode.com/robots.txt](https://leetcode.com/robots.txt) | 295 ms | 547 ms | 468 ms |
| site-944 | umbrel.com | [https://umbrel.com/robots.txt](https://umbrel.com/robots.txt) | 173 ms | 423 ms | 224 ms |
| site-945 | ibis-project.org | [https://ibis-project.org/robots.txt](https://ibis-project.org/robots.txt) | 142 ms | 491 ms | 209 ms |
| site-946 | www.tohoku.ac.jp | [https://www.tohoku.ac.jp/robots.txt](https://www.tohoku.ac.jp/robots.txt) | 935 ms | 839 ms | 949 ms |
| site-947 | www.hokudai.ac.jp | [https://www.hokudai.ac.jp/robots.txt](https://www.hokudai.ac.jp/robots.txt) | 620 ms | 294 ms | 305 ms |
| site-948 | proxmox.com | [https://proxmox.com/robots.txt](https://proxmox.com/robots.txt) | 614 ms | 618 ms | 348 ms |
| site-949 | pandas.pydata.org | [https://pandas.pydata.org/robots.txt](https://pandas.pydata.org/robots.txt) | 184 ms | 213 ms | 210 ms |
| site-950 | www.aist.go.jp | [https://www.aist.go.jp/robots.txt](https://www.aist.go.jp/robots.txt) | 201 ms | 496 ms | 195 ms |
| site-951 | dev.to | [https://dev.to/robots.txt](https://dev.to/robots.txt) | 163 ms | 195 ms | 192 ms |
| site-952 | hashnode.com | [https://hashnode.com/robots.txt](https://hashnode.com/robots.txt) | 231 ms | 225 ms | 230 ms |
| site-953 | www.jaxa.jp | [https://www.jaxa.jp/robots.txt](https://www.jaxa.jp/robots.txt) | 198 ms | 604 ms | 244 ms |
| site-954 | medium.com | [https://medium.com/robots.txt](https://medium.com/robots.txt) | 152 ms | 201 ms | 208 ms |
| site-955 | www.nims.go.jp | [https://www.nims.go.jp/robots.txt](https://www.nims.go.jp/robots.txt) | 244 ms | 287 ms | 306 ms |
| site-956 | www.sorbonne-universite.fr | [https://www.sorbonne-universite.fr/robots.txt](https://www.sorbonne-universite.fr/robots.txt) | 138 ms | 188 ms | 190 ms |
| site-958 | jax.readthedocs.io | [https://jax.readthedocs.io/robots.txt](https://jax.readthedocs.io/robots.txt) | 162 ms | 481 ms | 213 ms |
| site-959 | huggingface.co | [https://huggingface.co/robots.txt](https://huggingface.co/robots.txt) | 260 ms | 290 ms | 302 ms |
| site-960 | www.universite-paris-saclay.fr | [https://www.universite-paris-saclay.fr/robots.txt](https://www.universite-paris-saclay.fr/robots.txt) | 270 ms | 609 ms | 340 ms |
| site-961 | www.ens.psl.eu | [https://www.ens.psl.eu/robots.txt](https://www.ens.psl.eu/robots.txt) | 281 ms | 317 ms | 342 ms |
| site-962 | lobste.rs | [https://lobste.rs/robots.txt](https://lobste.rs/robots.txt) | 526 ms | 238 ms | 250 ms |
| site-963 | www.inria.fr | [https://www.inria.fr/robots.txt](https://www.inria.fr/robots.txt) | 269 ms | 317 ms | 344 ms |
| site-964 | www.polytechnique.edu | [https://www.polytechnique.edu/robots.txt](https://www.polytechnique.edu/robots.txt) | 316 ms | 357 ms | 383 ms |
| site-965 | wandb.ai | [https://wandb.ai/robots.txt](https://wandb.ai/robots.txt) | 184 ms | 238 ms | 241 ms |
| site-966 | buildah.io | [https://buildah.io/robots.txt](https://buildah.io/robots.txt) | 136 ms | 186 ms | 190 ms |
| site-967 | mlflow.org | [https://mlflow.org/robots.txt](https://mlflow.org/robots.txt) | 543 ms | 235 ms | 414 ms |
| site-968 | www.cnrs.fr | [https://www.cnrs.fr/robots.txt](https://www.cnrs.fr/robots.txt) | 802 ms | 611 ms | 329 ms |
| site-969 | buildkit.org | [https://buildkit.org/robots.txt](https://buildkit.org/robots.txt) | 161 ms | 514 ms | 216 ms |
| site-970 | www.pasteur.fr | [https://www.pasteur.fr/robots.txt](https://www.pasteur.fr/robots.txt) | 281 ms | 324 ms | 337 ms |
| site-971 | www.inserm.fr | [https://www.inserm.fr/robots.txt](https://www.inserm.fr/robots.txt) | 305 ms | 354 ms | 332 ms |
| site-973 | www.inrae.fr | [https://www.inrae.fr/robots.txt](https://www.inrae.fr/robots.txt) | 134 ms | 188 ms | 201 ms |
| site-974 | earthly.dev | [https://earthly.dev/robots.txt](https://earthly.dev/robots.txt) | 144 ms | 812 ms | 218 ms |
| site-975 | dagger.io | [https://dagger.io/robots.txt](https://dagger.io/robots.txt) | 492 ms | 501 ms | 199 ms |
| site-976 | realpython.com | [https://realpython.com/robots.txt](https://realpython.com/robots.txt) | 597 ms | 858 ms | 285 ms |
| site-977 | depot.dev | [https://depot.dev/robots.txt](https://depot.dev/robots.txt) | 137 ms | 190 ms | 205 ms |
| site-978 | testdriven.io | [https://testdriven.io/robots.txt](https://testdriven.io/robots.txt) | 170 ms | 213 ms | 233 ms |
| site-979 | www.fraunhofer.de | [https://www.fraunhofer.de/robots.txt](https://www.fraunhofer.de/robots.txt) | 670 ms | 721 ms | 369 ms |
| site-980 | www.cea.fr | [https://www.cea.fr/robots.txt](https://www.cea.fr/robots.txt) | 293 ms | 332 ms | 367 ms |
| site-981 | greatexpectations.io | [https://greatexpectations.io/robots.txt](https://greatexpectations.io/robots.txt) | 192 ms | 539 ms | 225 ms |
| site-982 | dvc.org | [https://dvc.org/robots.txt](https://dvc.org/robots.txt) | 379 ms | 190 ms | 189 ms |
| site-983 | deepchecks.com | [https://deepchecks.com/robots.txt](https://deepchecks.com/robots.txt) | 195 ms | 610 ms | 254 ms |
| site-984 | helmfile.readthedocs.io | [https://helmfile.readthedocs.io/robots.txt](https://helmfile.readthedocs.io/robots.txt) | 157 ms | 225 ms | 208 ms |
| site-985 | www.daad.de | [https://www.daad.de/robots.txt](https://www.daad.de/robots.txt) | 291 ms | 332 ms | 369 ms |
| site-986 | lakefs.io | [https://lakefs.io/robots.txt](https://lakefs.io/robots.txt) | 173 ms | 782 ms | 190 ms |
| site-987 | www.dfg.de | [https://www.dfg.de/robots.txt](https://www.dfg.de/robots.txt) | 648 ms | 634 ms | 336 ms |
| site-988 | webglfundamentals.org | [https://webglfundamentals.org/robots.txt](https://webglfundamentals.org/robots.txt) | 527 ms | 494 ms | 208 ms |
| site-989 | fluxcd.io | [https://fluxcd.io/robots.txt](https://fluxcd.io/robots.txt) | 150 ms | 479 ms | 196 ms |
| site-990 | www.desy.de | [https://www.desy.de/robots.txt](https://www.desy.de/robots.txt) | 640 ms | 365 ms | 385 ms |
| site-991 | babylonjs.com | [https://babylonjs.com/robots.txt](https://babylonjs.com/robots.txt) | 157 ms | 186 ms | 191 ms |
| site-992 | argo-cd.readthedocs.io | [https://argo-cd.readthedocs.io/robots.txt](https://argo-cd.readthedocs.io/robots.txt) | 151 ms | 235 ms | 429 ms |
| site-993 | godotengine.org | [https://godotengine.org/robots.txt](https://godotengine.org/robots.txt) | 145 ms | 204 ms | 214 ms |
| site-994 | www.gsi.de | [https://www.gsi.de/robots.txt](https://www.gsi.de/robots.txt) | 274 ms | 356 ms | 338 ms |
| site-995 | clear.ml | [https://clear.ml/robots.txt](https://clear.ml/robots.txt) | 160 ms | 202 ms | 203 ms |
| site-996 | tekton.dev | [https://tekton.dev/robots.txt](https://tekton.dev/robots.txt) | 149 ms | 485 ms | 197 ms |
| site-998 | www.embl.org | [https://www.embl.org/robots.txt](https://www.embl.org/robots.txt) | 277 ms | 325 ms | 324 ms |
| site-999 | www.embo.org | [https://www.embo.org/robots.txt](https://www.embo.org/robots.txt) | 283 ms | 647 ms | 339 ms |
| site-1000 | love2d.org | [https://love2d.org/robots.txt](https://love2d.org/robots.txt) | 166 ms | 514 ms | 213 ms |
| site-1001 | concourse-ci.org | [https://concourse-ci.org/robots.txt](https://concourse-ci.org/robots.txt) | 135 ms | 187 ms | 187 ms |
| site-1002 | www.europeanspallationsource.se | [https://www.europeanspallationsource.se/robots.txt](https://www.europeanspallationsource.se/robots.txt) | 722 ms | 972 ms | 396 ms |
| site-1003 | xgboost.ai | [https://xgboost.ai/robots.txt](https://xgboost.ai/robots.txt) | 139 ms | 194 ms | 216 ms |
| site-1004 | buildkite.com | [https://buildkite.com/robots.txt](https://buildkite.com/robots.txt) | 509 ms | 185 ms | 194 ms |
| site-1005 | lightgbm.readthedocs.io | [https://lightgbm.readthedocs.io/robots.txt](https://lightgbm.readthedocs.io/robots.txt) | 169 ms | 212 ms | 218 ms |

## 因规模限制移除的 18 个地址

以下地址均通过三家测试，仅因上述排序落在 470 名之后而移除。

| 探查地址 | 九次最大延迟 | 同出口最大轮间范围 |
| --- | ---: | ---: |
| [https://clang.llvm.org/robots.txt](https://clang.llvm.org/robots.txt) | 1059 ms | 358 ms |
| [https://www.duke.edu/robots.txt](https://www.duke.edu/robots.txt) | 1080 ms | 592 ms |
| [https://vercel.com/robots.txt](https://vercel.com/robots.txt) | 1084 ms | 913 ms |
| [https://llvm.org/robots.txt](https://llvm.org/robots.txt) | 1085 ms | 436 ms |
| [https://optuna.org/robots.txt](https://optuna.org/robots.txt) | 1092 ms | 912 ms |
| [https://creativecommons.org/robots.txt](https://creativecommons.org/robots.txt) | 1105 ms | 924 ms |
| [https://www.ebay.com/robots.txt](https://www.ebay.com/robots.txt) | 1193 ms | 1000 ms |
| [https://www.lse.ac.uk/robots.txt](https://www.lse.ac.uk/robots.txt) | 1228 ms | 330 ms |
| [https://www.openbsd.org/robots.txt](https://www.openbsd.org/robots.txt) | 1250 ms | 999 ms |
| [https://www.amd.com/robots.txt](https://www.amd.com/robots.txt) | 1280 ms | 1128 ms |
| [https://www.umn.edu/robots.txt](https://www.umn.edu/robots.txt) | 1284 ms | 1126 ms |
| [https://www.psl.eu/robots.txt](https://www.psl.eu/robots.txt) | 1331 ms | 605 ms |
| [https://bentoml.com/robots.txt](https://bentoml.com/robots.txt) | 1335 ms | 1102 ms |
| [https://www.gla.ac.uk/robots.txt](https://www.gla.ac.uk/robots.txt) | 1337 ms | 997 ms |
| [https://www.berkeley.edu/robots.txt](https://www.berkeley.edu/robots.txt) | 1386 ms | 1186 ms |
| [https://www.kcl.ac.uk/robots.txt](https://www.kcl.ac.uk/robots.txt) | 1398 ms | 372 ms |
| [https://pypi.org/robots.txt](https://pypi.org/robots.txt) | 1423 ms | 1292 ms |
| [https://www.nottingham.ac.uk/robots.txt](https://www.nottingham.ac.uk/robots.txt) | 1620 ms | 512 ms |
