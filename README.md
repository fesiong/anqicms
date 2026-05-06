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

### 快速部署

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

## 核心功能

### 多站点与多语言
- **多站点管理**：支持管理多个独立站点，适用于多品牌、多主题网站
- **多语言站点支持**：支持多语言站点配置和切换，支持整页HTML翻译，适合国际化需求

### SEO 优化工具
- **伪静态规则管理**：支持自定义伪静态 URL，优化搜索引擎收录
- **Sitemap 自动生成**：自动生成站点 Sitemap，便于搜索引擎抓取
- **链接推送管理**：向搜索引擎推送新内容，加速收录（支持百度/Bing主动推送）
- **301 跳转管理**：设置 URL 重定向，提升 SEO 和用户体验
- **Robots.txt 配置**：定制 Robots.txt 文件，控制搜索引擎爬取
- **锚文本管理**：设置站内关键词的自动锚文本链接
- **关键词库管理**：集中管理 SEO 关键词

### 内容管理与安全
- **自定义内容模型**：定义特定内容模型，支持灵活的内容结构
- **内容安全设置**：敏感词过滤和内容审核，确保内容合规
- **防采集干扰码**：生成干扰码，保护内容不被批量采集
- **文档回收站**：支持已删除文档的恢复，防止数据误删
- **备份与恢复**：支持数据备份和恢复，保障数据安全

### AI 与自动化
- **AI 写作功能**：接入AI自动写作，辅助内容创作（2023年4月起）
- **LLMs.txt 支持**：自动生成站点 LLMs.txt 文件，便于大语言模型理解和索引网站内容
- **结构化数据**：支持 JSON-LD 等结构化数据输出，提升搜索引擎对页面内容的理解
- **批量导入文档**：支持 ZIP 压缩包和 Excel 表格批量导入文档，适合大规模内容迁移
- **时间因子-定时发布**：设定内容定时发布，支持自动运营
- **内容采集**：支持从其他网站自动采集内容
- **标题自动配图**：根据标题内容自动分配图片

### 用户与商业功能
- **用户管理和 VIP 分组**：为不同用户组设置访问权限
- **微信公众号对接**：集成微信公众号接口
- **订单管理**：内置订单管理功能
- **财务管理模块**：记录财务数据，管理交易明细
- **分销管理**：设置分销渠道

### 其他工具
- **全局设置**：集中管理系统各项配置
- **导航设置**：管理导航栏
- **单页面管理**：隐私政策、关于我们等单页面工具
- **全站替换工具**：批量替换关键词或链接
- **邮件提醒**：根据触发事件发送邮件通知
- **水印管理**：为图片添加水印，保护版权
- **静态页面缓存**：提高页面访问速度，减少服务器负载
- **流量统计和蜘蛛统计**：追踪访问量及搜索引擎爬虫抓取情况

## AnQiCMS 发展历程

AnQiCMS 起源于2019年的GoBlog项目，历经多次重构和功能扩展，于2022年5月正式更名为AnQiCMS，逐步发展为面向企业级应用的内容管理系统。

- **最新**
  > 持续迭代优化，探索AI内容创作与CMS的深度融合
- **2025年 03月 17日，v3.4.7 发布**
  > 多语言增强，支持整页HTML翻译
- **2024年 11月 11日，v3.4.1 发布**
  > 新增多语言站点功能，提升全球化服务能力
- **2024年 9月 5日，v3.3.12 发布**
  > 优化文档查询效率，降低资源消耗，提升大文章量场景性能
- **2024年 5月 1日，v3.3.5 发布**
  > 支持图片水印功能
- **2023年 10月 24日，v3.2.5 发布**
  > 支持多语言翻译功能，支持 Markdown 编辑器
- **2023年 4月 15日，v3.1.1 发布**
  > 接入AI自动写作功能
- **2022年 12月 5日，v3.0.0 发布**
  > 支持多站点模式，简化部署难度，新增企业站常用功能
- **2022年 5月 30日，v2.1.0 发布**
  > 正式更名为 AnQiCMS，具备常用内容管理系统功能
- **2021年 2月 16日，v2.0.0-alpha 发布**
  > 由博客系统向内容管理系统转型
- **2021年 1月 21日，GoBlog v1.0.0 发布**  
  > 博客完善版，增加后台管理、SEO功能
- **2020年 12月 1日，GoBlog v0.5 发布**  
  > 采用Iris框架重写，实现基础博客功能
- **2019年 11月 19日，GoBlog v0.1 发布**   
  > Gin版本，前后端分离架构

## 技术依赖

- [gorm](https://github.com/go-gorm/gorm) - ORM框架
- [iris](https://github.com/kataras/iris) - Web框架
- [jwt](https://github.com/golang-jwt/jwt) - JWT认证
- [sego](https://github.com/huichen/sego) - 中文分词
- [gorequest](https://github.com/parnurzeal/gorequest) - HTTP请求库
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML解析
- [chromedp](https://github.com/chromedp/chromedp) - 浏览器自动化
- [markdown](https://github.com/gomarkdown/markdown) - Markdown解析
- [webp](https://github.com/chai2010/webp) - WebP图片处理
- [cron](https://github.com/robfig/cron) - 定时任务
- [open-golang](https://github.com/skratchdot/open-golang) - 文件打开工具
- [go-qrcode](https://github.com/skip2/go-qrcode) - 二维码生成

## 管理后台访问

如果你从 GitHub 克隆代码并自行编译运行，需要先编译后台管理代码，后台管理代码在 https://github.com/fesiong/anqicms-admin 。

你也可以从后台管理代码的release中下载最新版本，将 `system.zip` 解压到项目根目录下的 `system` 文件夹。

后台地址默认为 `http://127.0.0.1:8001/system`

默认管理员账号（非初始化安装）：

账号：`admin`

密码：`123456`

## 文档与示例

[示例网站](https://www.anqicms.com/manual) · [开发文档](https://www.anqicms.com/help)

## 问题反馈

遇到问题，请在Github上开issue。

也可以加微信：`websafety`，扫码加入golang开发学习群

![扫码入群讨论](https://www.anqicms.com/uploads/202211/09/1a55bfcde55aa2d6.webp)

## License

AGPL-3.0 license

Copyright (c) 2019-NOW  AnQiCMS <tpyzlxy@gmail.com>
