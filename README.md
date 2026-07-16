<div align="center"><a name="readme-top"></a>

<img height="180" src="docs/anqicms.svg" />

# AnQiCMS 基于Go语言的企业级内容管理系统

<div align="center">
<a href="https://github.com/fesiong/anqicms/releases"><img alt="GitHub release" src="https://img.shields.io/github/release/fesiong/anqicms.svg?style=flat-square&include_prereleases" /></a>
<a href="https://hub.docker.com/r/anqicms/anqicms"><img alt="Docker pulls" src="https://img.shields.io/docker/pulls/anqicms/anqicms?style=flat-square" /></a>
<a href="https://github.com/fesiong/anqicms/commits"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/fesiong/anqicms.svg?style=flat-square" /></a>
  </div>

[官网](https://www.anqicms.com) · [文档](https://www.anqicms.com/help) · [演示](https://demo.anqicms.com) · [GitCode](https://gitcode.com/anqicms/anqicms) · [更新日志](./CHANGELOG.md) · [English](./README-en_US.md) · 中文

</div>

## 关于 AnQiCMS

AnQiCMS（安企CMS）是一款基于GoLang开发的企业级内容管理系统，前身是GoBlog，以"安全"和"企业级应用"为核心优势。系统采用Go语言的高性能架构，内存占用比PHP类CMS降低约80%，支持多站点、多语言管理及AI内容创作，适用于中小型企业官网、营销型网站、政府门户、跨境电商站等场景。

**核心技术特点：**
- **技术栈**：GoLang + Iris框架 + GORM
- **性能表现**：页面加载速度相比传统PHP CMS有显著提升，单机可承载约500万PV
- **安全机制**：内置JWT认证、内容敏感词过滤、防采集干扰码，有效防御SQL注入和XSS攻击
- **SEO支持**：伪静态URL、301重定向、Sitemap自动生成、百度/Bing主动推送、锚文本管理

## 为什么选择 Go 语言 CMS

传统的 CMS 系统在大规模内容管理场景下面临服务器资源占用高、页面加载缓慢等问题。PHP 开发的 CMS 系统存在安全隐患，网站挂马、入侵等问题时有发生。AnQiCMS 利用Go语言的并发优势和编译型特性，提供更稳定的运行环境和更低的资源消耗，同时通过内置的安全机制降低网站被攻击的风险。

**品牌愿景：让天下都是安全的网站**

## 快速开始

### 快速(宝塔)部署

- [宝塔面板(9.2+)一键部署**推荐**](https://www.anqicms.com/help/3628.html)
- [aaPanel(宝塔国际版)一键部署](https://www.anqicms.com/help/3633.html)
- [宝塔面板(9.0以前版本)部署](https://www.anqicms.com/help-basic/210.html)
- [LNMP命令部署](https://www.anqicms.com/courses/3486.html)

Docker 快速体验

```bash
docker pull anqicms/anqicms:latest
docker run -d --name anqicms -p 8001:8001 anqicms/anqicms:latest
```

### 在线体验

环境地址：[https://demo.anqicms.com](https://demo.anqicms.com)  
后台地址：[https://demo.anqicms.com/system](https://demo.anqicms.com/system)  
用户名：`admin`  
密码：`123456`  

### 使用帮助

[查看模板使用教程](https://www.anqicms.com/manual)  
[查看后台使用帮助](https://www.anqicms.com/help)  
[查看接口文档](https://www.anqicms.com/anqiapi)

> 温馨提示：大多数文档的编写语言为中文，如果您是英文用户，请使用浏览器自带翻译功能自行翻译。

## 网站特色功能

- **全局设置**：提供站点的统一配置，集中管理系统各项全局设置。
- **内容安全设置**：包括敏感词过滤和内容审核，确保内容合规。
- **多站点管理**：支持管理多个独立站点，适用于多品牌、多主题网站。
- **导航设置**：设置和管理导航栏，提高用户导航体验。
- **自定义内容模型**：允许用户定义特定内容模型，支持灵活的内容结构。
- **单页面管理**：针对隐私政策、关于我们等单页面的专用管理工具。
- **伪静态规则管理**：支持自定义伪静态 URL，优化 SEO 表现。
- **Sitemap 自动生成**：自动生成站点的 Sitemap，助力搜索引擎抓取。
- **链接推送管理**：向搜索引擎推送新内容，加速收录。
- **301 跳转管理**：轻松设置 URL 的 301 重定向，提升 SEO 和用户体验。
- **Robots.txt 配置**：定制 Robots.txt 文件，控制搜索引擎爬取。
- **多语言站点支持**：支持多语言站点的配置和切换，适合国际化站点。
- **用户管理和 VIP 分组**：可为不同用户组设置访问权限，适合会员制网站。
- **微信公众号对接**：集成微信公众号接口，便于内容推送和管理。
- **全站替换工具**：可批量替换关键词或链接，方便内容批量更新。
- **邮件提醒**：根据触发事件（如注册、评论）发送邮件通知，提升用户参与度。
- **关键词库管理**：集中管理 SEO 关键词，有助于内容优化。
- **内容采集**：支持从其他网站自动采集内容，实现内容自动化更新。
- **文档回收站**：支持已删除文档的恢复，防止数据误删。
- **时间因子-定时发布**：可设定内容的定时发布，支持内容自动运营。
- **订单管理**：内置订单管理功能，适合电商和服务付费类站点。
- **财务管理模块**：记录财务数据，管理交易明细，适合订阅、交易网站。
- **分销管理**：可设置分销渠道，帮助站点实现裂变增长。
- **防采集干扰码**：生成干扰码，保护内容不被批量采集。
- **锚文本管理**：可设置站内关键词的自动锚文本链接，有助于 SEO。
- **水印管理**：为图片添加水印，保护原创图片版权。
- **标题自动配图**：根据标题内容自动分配图片，提升内容视觉效果。
- **静态页面缓存**：提高页面访问速度，减少服务器负载。
- **备份与恢复**：系统支持数据的备份和恢复，保障数据安全。
- **流量统计和蜘蛛统计**：追踪网站的访问量及搜索引擎爬虫的抓取情况，便于数据分析。

## AnQiCMS 发展历程

- 最新
  > 我们一直在努力，不断的探索，持续迭代优化，争取给用户们有更好的使用体验。
- 2025年 03月 17日，v3.4.7 发布
  > 多语言站点‌：多语言增强，支持整页HTML翻译
- 2024年 11月 11日，v3.4.1 发布
  > 多语言站点‌：新增多语言站点功能，使网站能够轻松适应不同地区和文化背景用户需求，提升全球化服务能力
- 2024年 9月 5日，v3.3.12 发布
  > 优化了文档查询效率，降低了资源消耗，提升了网站性能，特别针对大文章量场景
- 2024年 5月 1日，v3.3.5 发布
  > 支持图片水印功能
- 2023年10月24日，v3.2.5 发布
  > 支持多语言翻译功能，支持 Markdown 编辑器
- 2023年 4月15日，v3.1.1 发布
  > 接入AI自动写作功能
- 2022年12月 5日，v3.0.0 发布
  > 开始支持多站点模式，简化和降低了 AnQiCMS 的部署难度，新增更多丰富的企业站常用功能
- 2022年 5月30日，v2.1.0 发布
  > 正式更名为 AnQiCMS，标志着 AnQiCMS 已具备常用的内容管理系统必备功能
- 2021年 2月16日，v2.0.0-alpha 发布
  > 开始逐步由单纯的博客功能，过度到更全面的内容管理系统，逐步开发并完善企业站功能
- 2021年 1月21日，GoBlog v1.0.0 发布  
  > 博客完善版，在基础版的基础上，增加了后台管理、seo功能等。
- 2020年12月 1日，GoBlog v0.5 发布  
  > 重构版本，采用iris框架重写，减少技术栈，改用iris自带的template模板引擎。实现了最基础的博客功能。
- 2019年11月19日 GoBlog v0.1 发布   
  > Gin版本，前后端分离，后端使用go、gin、gorm，前端使用Next.js。

## 使用的包

- [gorm](https://github.com/go-gorm/gorm)
- [iris](https://github.com/kataras/iris)
- [jwt](https://github.com/golang-jwt/jwt)
- [sego](https://github.com/huichen/sego)
- [gorequest](https://github.com/parnurzeal/gorequest)
- [goquery](https://github.com/PuerkitoBio/goquery)
- [chromedp](https://github.com/chromedp/chromedp)
- [markdown](https://github.com/gomarkdown/markdown)
- [webp](https://github.com/chai2010/webp)
- [cron](https://github.com/robfig/cron)
- [open-golang](https://github.com/skratchdot/open-golang)
- [go-qrcode](https://github.com/skip2/go-qrcode)

## 访问管理后台
如果你从 GitHub 上克隆下载的代码，自行编译运行的话，需要先编译后台的管理代码，后台管理代码在 https://github.com/fesiong/anqicms-admin 。
你也可以从 后台管理代码的release中，下载最新的release，将system.zip 解压到项目根目录下的system文件夹。

后台地址默认为 http://127.0.0.1:8001/system

如果你不是通过安装初始化的话，可能没有设置管理员账号，如果没有设置管理员账号，默认的管理员账号密码分别是：

账号：admin

密码：123456

## 示例网站 & 开发文档
[示例网站 - https://www.anqicms.com/](https://www.anqicms.com/manual)


## 👥问题反馈    
遇到问题, 请在Github上开issue。  
也可以加我的微信：websafety

扫码加入golang开发学习群

![扫码入群讨论](https://www.anqicms.com/uploads/202211/09/1a55bfcde55aa2d6.webp)

## License
AGPL-3.0 license

Copyright (c) 2019-NOW  AnQiCMS <tpyzlxy@gmail.com>
