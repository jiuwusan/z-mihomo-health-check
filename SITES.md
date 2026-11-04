# 知名大厂精选探查站点池（54 个）

更新日期：2026-11-04（Asia/Shanghai）。仅纳入大型科技、互联网、云软件或网络安全公司的官方服务域名；移除此前 Debian、Mozilla、PyPI、W3C、IETF、curl、jsDelivr、UNPKG 等非本次大厂范围的端点。

## 筛选与分散原则

- 优先 Google、Cloudflare、Apple 的专用连通性检测端点，其他候选使用官方 robots.txt 等简单资源，避免业务首页、登录和动态搜索入口。
- 先从工作机发起 HTTPS HEAD，不跟随重定向，须直接返回 2xx；该结构检查与 Mihomo 使用不同流量路径，不能代表被测出口的最终 HTTP 状态。
- 通过指定 Mihomo 服务逐个测试，每个合格端点 5 轮；原 50 个端点轮间等待 10 秒，Docker 新增端点先连续测试 3 次，再补测 2 轮（补测轮间等待 10 秒），timeout=4000、expected=200-299。每轮必须 API HTTP 200、delay>0 且 delay≤2000 ms，5 轮全部通过才可入选。
- 分两批完成，共检查 132 个候选，72 个通过；在合格项中按厂商分散选取 50 个，覆盖 41 个服务方分组，原有服务方最多 2 个地址；按用户要求追加 Docker 的 4 个端点。服务方分组用于控制集中度，不是经过法律核验的独立公司数量；不同公司也可能共用 CDN。
- 50 个地址共 250 次 Mihomo 测试成功，最大记录延迟 1481 ms。两批读取到的目标分组均为 🚀 节点选择，选择均为 🇺🇸 E-IX ANYTLS 美国；每轮前后校验选择一致，没有发送切换请求。分组内部实际节点可能由 Mihomo 自动决定。

大厂身份不等于探查服务的 SLA。这里的“精选”是基于端点结构和短时间测试的筛选，尚未完成跨天、多出口或多地区验证，不承诺长期可用率，也不保证请求落点一定在境外。

## 文件与运行规则

- [sites.txt](./sites.txt)：运行时嵌入的 54 行 HTTPS URL。
- [sites.json](./sites.json)：对应编号、名称、服务方、类型和 expectedStatus。
- [probe-results.json](./probe-results.json)：全部候选的结构检查、五轮记录和最终 selected 标记，不含连接凭据。

已有 URL 保留编号，新 URL 从 site-516 起，移除编号不复用。GNU、FSF、CBC 不在当前池。

默认每 5 秒均匀随机抽取一个站点，每站长期平均约 270 秒（4 分 30 秒）被选中一次，不保证最短重访间隔。整组测试、恢复检查和订阅自身探查会增加请求量；此次不修改实例的超时、间隔、阈值等运行配置。

Mihomo delay API 的 HTTP 状态和正延迟不等于目标网站的 HTTP 状态，因此同时采用单独的 HEAD 结构筛选；没有从指定出口额外读取目标响应正文或响应状态。

Docker 补充验证：8 个候选中 4 个符合条件，追加后共 54 个端点、42 个服务方分组、270 次入选探查成功。Docker 的 4 个端点均通过 5 次 4 秒限时探查，最大延迟 493 ms；Registry 的 401 及其他 404 地址未加入。原 50 个端点的筛选统计已保留，新增记录已并入探测快照。

## 当前清单

| 编号 | 服务方 | 名称 | 探查地址 | HEAD 结构检查 | 5 轮最大延迟 |
| --- | --- | --- | --- | ---: | ---: |
| site-504 | Google | Google · www.gstatic.com | [https://www.gstatic.com/generate_204](https://www.gstatic.com/generate_204) | 204 | 151 ms |
| site-506 | Apple | Apple · captive.apple.com | [https://captive.apple.com/hotspot-detect.html](https://captive.apple.com/hotspot-detect.html) | 200 | 139 ms |
| site-509 | Microsoft | Microsoft · raw.githubusercontent.com | [https://raw.githubusercontent.com/github/gitignore/main/README.md](https://raw.githubusercontent.com/github/gitignore/main/README.md) | 200 | 139 ms |
| site-516 | Amazon | Amazon · www.aboutamazon.com | [https://www.aboutamazon.com/robots.txt](https://www.aboutamazon.com/robots.txt) | 200 | 138 ms |
| site-517 | Meta | Meta · www.facebook.com | [https://www.facebook.com/robots.txt](https://www.facebook.com/robots.txt) | 200 | 226 ms |
| site-518 | Cloudflare | Cloudflare · developers.cloudflare.com | [https://developers.cloudflare.com/robots.txt](https://developers.cloudflare.com/robots.txt) | 200 | 262 ms |
| site-519 | AMD | AMD · www.amd.com | [https://www.amd.com/robots.txt](https://www.amd.com/robots.txt) | 200 | 221 ms |
| site-520 | SAP | SAP · help.sap.com | [https://help.sap.com/robots.txt](https://help.sap.com/robots.txt) | 200 | 152 ms |
| site-521 | IBM | IBM · www.redhat.com | [https://www.redhat.com/robots.txt](https://www.redhat.com/robots.txt) | 200 | 175 ms |
| site-522 | Salesforce | Salesforce · www.heroku.com | [https://www.heroku.com/robots.txt](https://www.heroku.com/robots.txt) | 200 | 138 ms |
| site-523 | Dropbox | Dropbox · help.dropbox.com | [https://help.dropbox.com/robots.txt](https://help.dropbox.com/robots.txt) | 200 | 170 ms |
| site-524 | Box | Box · www.box.com | [https://www.box.com/robots.txt](https://www.box.com/robots.txt) | 200 | 172 ms |
| site-525 | Stripe | Stripe · stripe.com | [https://stripe.com/robots.txt](https://stripe.com/robots.txt) | 200 | 503 ms |
| site-526 | Shopify | Shopify · help.shopify.com | [https://help.shopify.com/robots.txt](https://help.shopify.com/robots.txt) | 200 | 279 ms |
| site-527 | Booking Holdings | Booking Holdings · www.booking.com | [https://www.booking.com/robots.txt](https://www.booking.com/robots.txt) | 200 | 506 ms |
| site-528 | eBay | eBay · www.ebay.com | [https://www.ebay.com/robots.txt](https://www.ebay.com/robots.txt) | 200 | 733 ms |
| site-529 | Broadcom | Broadcom · www.broadcom.com | [https://www.broadcom.com/robots.txt](https://www.broadcom.com/robots.txt) | 200 | 220 ms |
| site-530 | Nokia | Nokia · www.nokia.com | [https://www.nokia.com/robots.txt](https://www.nokia.com/robots.txt) | 200 | 396 ms |
| site-531 | Snowflake | Snowflake · www.snowflake.com | [https://www.snowflake.com/robots.txt](https://www.snowflake.com/robots.txt) | 200 | 161 ms |
| site-532 | Datadog | Datadog · www.datadoghq.com | [https://www.datadoghq.com/robots.txt](https://www.datadoghq.com/robots.txt) | 200 | 141 ms |
| site-533 | CrowdStrike | CrowdStrike · www.crowdstrike.com | [https://www.crowdstrike.com/robots.txt](https://www.crowdstrike.com/robots.txt) | 200 | 232 ms |
| site-534 | Zscaler | Zscaler · www.zscaler.com | [https://www.zscaler.com/robots.txt](https://www.zscaler.com/robots.txt) | 200 | 296 ms |
| site-535 | Okta | Okta · www.okta.com | [https://www.okta.com/robots.txt](https://www.okta.com/robots.txt) | 200 | 171 ms |
| site-536 | Intuit | Intuit · www.intuit.com | [https://www.intuit.com/robots.txt](https://www.intuit.com/robots.txt) | 200 | 363 ms |
| site-537 | GitLab | GitLab · about.gitlab.com | [https://about.gitlab.com/robots.txt](https://about.gitlab.com/robots.txt) | 200 | 172 ms |
| site-538 | Elastic | Elastic · www.elastic.co | [https://www.elastic.co/robots.txt](https://www.elastic.co/robots.txt) | 200 | 195 ms |
| site-539 | MongoDB | MongoDB · www.mongodb.com | [https://www.mongodb.com/robots.txt](https://www.mongodb.com/robots.txt) | 200 | 478 ms |
| site-540 | DigitalOcean | DigitalOcean · www.digitalocean.com | [https://www.digitalocean.com/robots.txt](https://www.digitalocean.com/robots.txt) | 200 | 722 ms |
| site-541 | OVHcloud | OVHcloud · www.ovhcloud.com | [https://www.ovhcloud.com/robots.txt](https://www.ovhcloud.com/robots.txt) | 200 | 161 ms |
| site-542 | Atlassian | Atlassian · developer.atlassian.com | [https://developer.atlassian.com/robots.txt](https://developer.atlassian.com/robots.txt) | 200 | 183 ms |
| site-543 | SUSE | SUSE · www.suse.com | [https://www.suse.com/robots.txt](https://www.suse.com/robots.txt) | 200 | 182 ms |
| site-544 | Gen Digital | Gen Digital · www.avast.com | [https://www.avast.com/robots.txt](https://www.avast.com/robots.txt) | 200 | 155 ms |
| site-545 | ESET | ESET · www.eset.com | [https://www.eset.com/robots.txt](https://www.eset.com/robots.txt) | 200 | 586 ms |
| site-546 | Kaspersky | Kaspersky · www.kaspersky.com | [https://www.kaspersky.com/robots.txt](https://www.kaspersky.com/robots.txt) | 200 | 320 ms |
| site-547 | Adobe | Adobe · developer.adobe.com | [https://developer.adobe.com/robots.txt](https://developer.adobe.com/robots.txt) | 200 | 142 ms |
| site-548 | Oracle | Oracle · docs.oracle.com | [https://docs.oracle.com/robots.txt](https://docs.oracle.com/robots.txt) | 200 | 632 ms |
| site-549 | NVIDIA | NVIDIA · docs.nvidia.com | [https://docs.nvidia.com/robots.txt](https://docs.nvidia.com/robots.txt) | 200 | 878 ms |
| site-550 | Cisco | Cisco · developer.cisco.com | [https://developer.cisco.com/robots.txt](https://developer.cisco.com/robots.txt) | 200 | 187 ms |
| site-551 | Dell | Dell · www.delltechnologies.com | [https://www.delltechnologies.com/robots.txt](https://www.delltechnologies.com/robots.txt) | 200 | 1481 ms |
| site-552 | PayPal | PayPal · developer.paypal.com | [https://developer.paypal.com/robots.txt](https://developer.paypal.com/robots.txt) | 200 | 562 ms |
| site-553 | Zoom | Zoom · developers.zoom.us | [https://developers.zoom.us/robots.txt](https://developers.zoom.us/robots.txt) | 200 | 141 ms |
| site-554 | Google | Google · www.google.com | [https://www.google.com/generate_204](https://www.google.com/generate_204) | 204 | 138 ms |
| site-555 | Apple | Apple · www.apple.com | [https://www.apple.com/robots.txt](https://www.apple.com/robots.txt) | 200 | 349 ms |
| site-556 | Microsoft | Microsoft · www.githubstatus.com | [https://www.githubstatus.com/robots.txt](https://www.githubstatus.com/robots.txt) | 200 | 140 ms |
| site-557 | Amazon | Amazon · aws.amazon.com | [https://aws.amazon.com/robots.txt](https://aws.amazon.com/robots.txt) | 200 | 169 ms |
| site-558 | Meta | Meta · www.whatsapp.com | [https://www.whatsapp.com/robots.txt](https://www.whatsapp.com/robots.txt) | 200 | 336 ms |
| site-559 | Cloudflare | Cloudflare · www.cloudflare.com | [https://www.cloudflare.com/robots.txt](https://www.cloudflare.com/robots.txt) | 200 | 725 ms |
| site-560 | SAP | SAP · www.sap.com | [https://www.sap.com/robots.txt](https://www.sap.com/robots.txt) | 200 | 909 ms |
| site-561 | IBM | IBM · www.ibm.com | [https://www.ibm.com/robots.txt](https://www.ibm.com/robots.txt) | 200 | 221 ms |
| site-562 | Salesforce | Salesforce · slack.com | [https://slack.com/robots.txt](https://slack.com/robots.txt) | 200 | 588 ms |
| site-563 | Docker | Docker 官网 robots | [https://www.docker.com/robots.txt](https://www.docker.com/robots.txt) | 200 | 493 ms |
| site-564 | Docker | Docker 文档 robots | [https://docs.docker.com/robots.txt](https://docs.docker.com/robots.txt) | 200 | 182 ms |
| site-565 | Docker | Docker Hub robots | [https://hub.docker.com/robots.txt](https://hub.docker.com/robots.txt) | 200 | 220 ms |
| site-566 | Docker | Docker 下载静态目录 | [https://download.docker.com/linux/static/stable/x86_64/](https://download.docker.com/linux/static/stable/x86_64/) | 200 | 139 ms |

## 参考

- [Mihomo 官方健康检查 URL 建议](https://wiki.metacubex.one/config/proxy-providers/)：Google、Cloudflare 专用地址。
- [Apple 企业网络说明](https://support.apple.com/en-us/101555)：captive.apple.com 用于网络连通性验证。
- 各厂商官方域名的直接 HEAD 结构检查和指定 Mihomo 服务的测试原始记录见探测快照。
- 控制器调度和故障转移规则见 [README.md](./README.md)。
