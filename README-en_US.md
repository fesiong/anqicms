<div align="center"><a name="readme-top"></a>

<img height="180" src="docs/anqicms.svg" />

# AnQiCMS: An Enterprise Content Management System Based on Go

<div align="center">
<a href="https://github.com/fesiong/anqicms/releases"><img alt="GitHub release" src="https://img.shields.io/github/release/fesiong/anqicms.svg?style=flat-square&include_prereleases" /></a>
<a href="https://hub.docker.com/r/anqicms/anqicms"><img alt="Docker pulls" src="https://img.shields.io/docker/pulls/anqicms/anqicms?style=flat-square" /></a>
<a href="https://github.com/fesiong/anqicms/commits"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/fesiong/anqicms.svg?style=flat-square" /></a>
</div>

[Official](https://www.anqicms.com) · [Docs](https://www.anqicms.com/help) · [Demo](https://demo.anqicms.com) · [GitCode](https://gitcode.com/anqicms/anqicms) · [Changelog](./CHANGELOG.md) · English · [中文](./README.md)

</div>

## About AnQiCMS

AnQiCMS is an enterprise-grade content management system developed in GoLang, evolved from the GoBlog project. It focuses on "security" and "enterprise applications" as its core advantages. Built with Go's high-performance architecture, it reduces memory usage by approximately 80% compared to PHP-based CMS solutions. It supports multi-site, multilingual management, and AI content creation, making it suitable for scenarios such as small and medium enterprise websites, marketing sites, government portals, and cross-border e-commerce platforms.

**Core Technical Features:**
- **Tech Stack**: GoLang + Iris Framework + GORM
- **Performance**: Significantly faster page loading compared to traditional PHP CMS, capable of handling approximately 5 million PV on a single server
- **Security**: Built-in JWT authentication, sensitive word filtering, and anti-crawling disruption codes to defend against SQL injection and XSS attacks
- **SEO Support**: Pseudo-static URLs, 301 redirects, automatic sitemap generation, Baidu/Bing active push, and anchor text management

## Why Choose a Go-Based CMS

Traditional CMS systems face challenges with high server resource consumption and slow page loading in large-scale content management scenarios. PHP-based CMS solutions are prone to security vulnerabilities, with frequent incidents of website defacement and unauthorized access. AnQiCMS leverages Go's concurrency advantages and compiled nature to provide a more stable runtime environment with lower resource consumption, while reducing website attack risks through built-in security mechanisms.

**Vision: To make every website secure.**

## Quick Start

### Quick Deployment

- [One-Click Deployment with aaPanel](https://www.anqicms.com/help/3633.html)
- [LNMP Command Deployment](https://www.anqicms.com/courses/3486.html)

Quick Start with Docker

```bash
docker pull anqicms/anqicms:latest
docker run -d --name anqicms -p 8001:8001 anqicms/anqicms:latest
```

### Demo Site

Frontend: [https://demo.anqicms.com](https://demo.anqicms.com)  
Backend: [https://demo.anqicms.com/system](https://demo.anqicms.com/system)  
User: `admin`  
Password: `123456`  

### Help

[Template Documentation](https://www.anqicms.com/manual)  
[Usage Guide](https://www.anqicms.com/help)  
[API Documentation](https://www.anqicms.com/anqiapi)

> Warm reminder: Most documents are written in Chinese. If you are an English user, please use the browser's built-in translation function to translate them yourself.

## Core Features

### Multi-Site & Multilingual
- **Multi-Site Management**: Supports managing multiple independent sites, ideal for multi-brand or multi-theme websites
- **Multilingual Site Support**: Enables configuration and switching of multilingual sites with full-page HTML translation, suitable for internationalization

### SEO Optimization Tools
- **Pseudo-Static Rule Management**: Supports custom pseudo-static URLs to optimize search engine indexing
- **Automatic Sitemap Generation**: Automatically generates a site sitemap to facilitate search engine crawling
- **Link Push Management**: Pushes new content to search engines to speed up indexing (supports Baidu/Bing active push)
- **301 Redirect Management**: Set up URL redirects to improve SEO and user experience
- **Robots.txt Configuration**: Customize the Robots.txt file to control search engine crawling
- **Anchor Text Management**: Configure automatic internal keyword linking for SEO
- **Keyword Library Management**: Centralized management of SEO keywords

### Content Management & Security
- **Custom Content Models**: Define specific content models for flexible content structures
- **Content Security Settings**: Sensitive word filtering and content review to ensure compliance
- **Anti-Crawling Disruption Code**: Generates disruption codes to protect content from bulk scraping
- **Document Recycle Bin**: Recover deleted documents to prevent accidental data loss
- **Backup and Recovery**: Supports data backup and recovery to ensure data security

### AI & Automation
- **AI Writing**: AI-powered content creation assistance (since April 2023)
- **LLMs.txt Support**: Automatically generates site LLMs.txt file for large language models to understand and index website content
- **Structured Data**: Supports JSON-LD and other structured data output to enhance search engine understanding of page content
- **Batch Document Import**: Supports batch importing documents via ZIP archives and Excel spreadsheets, suitable for large-scale content migration
- **Scheduled Publishing**: Set up timed content publishing for automated operations
- **Content Collection**: Automatic content collection from other websites
- **Automatic Title Images**: Automatically assigns images based on title content

### User & Business Features
- **User Management and VIP Grouping**: Set access permissions for different user groups
- **WeChat Official Account Integration**: Integrates with WeChat official accounts for content distribution
- **Order Management**: Built-in order management for e-commerce and service payment sites
- **Finance Management Module**: Records financial data and manages transaction details
- **Distribution Management**: Configure distribution channels for referral-based growth

### Other Tools
- **Global Settings**: Centralized management of all system configurations
- **Navigation Settings**: Manage navigation bars
- **Single Page Management**: Tools for privacy policies, "About Us" pages, etc.
- **Site-Wide Replacement Tool**: Batch replace keywords or links
- **Email Notifications**: Send email alerts triggered by events such as registration or comments
- **Watermark Management**: Add watermarks to images to protect copyrights
- **Static Page Caching**: Improve page load speed and reduce server load
- **Traffic and Spider Statistics**: Track website visits and search engine crawler activities

## AnQiCMS Development History

AnQiCMS originated from the GoBlog project in 2019. After multiple refactoring and feature expansions, it was officially renamed AnQiCMS in May 2022, gradually evolving into an enterprise-grade content management system.

- **Latest**
  > Continuous iteration and optimization, exploring deep integration of AI content creation with CMS
- **March 17, 2025, v3.4.7 Released**
  > Multilingual enhancement, supporting full-page HTML translation
- **November 11, 2024, v3.4.1 Released**
  > Introduced multi-language site support, enhancing global service capabilities
- **September 5, 2024, v3.3.12 Released**
  > Optimized document query efficiency, reduced resource consumption, improved performance for large-scale article scenarios
- **May 1, 2024, v3.3.5 Released**
  > Added image watermark support
- **October 24, 2023, v3.2.5 Released**
  > Added multilingual translation and Markdown editor support
- **April 15, 2023, v3.1.1 Released**
  > Integrated AI automatic writing functionality
- **December 5, 2022, v3.0.0 Released**
  > Multi-site mode support, simplified deployment, added enterprise site features
- **May 30, 2022, v2.1.0 Released**
  > Officially renamed AnQiCMS, with standard CMS functionality
- **February 16, 2021, v2.0.0-alpha Released**
  > Transitioning from a blog system to a content management system
- **January 21, 2021, GoBlog v1.0.0 Released**
  > Enhanced blog version with admin panel and SEO features
- **December 1, 2020, GoBlog v0.5 Released**
  > Rewritten with Iris framework, basic blog functionality
- **November 19, 2019, GoBlog v0.1 Released**
  > Gin-based version with front-end and back-end separation

## Technical Dependencies

- [gorm](https://github.com/go-gorm/gorm) - ORM Framework
- [iris](https://github.com/kataras/iris) - Web Framework
- [jwt](https://github.com/golang-jwt/jwt) - JWT Authentication
- [sego](https://github.com/huichen/sego) - Chinese Word Segmentation
- [gorequest](https://github.com/parnurzeal/gorequest) - HTTP Request Library
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML Parsing
- [chromedp](https://github.com/chromedp/chromedp) - Browser Automation
- [markdown](https://github.com/gomarkdown/markdown) - Markdown Parsing
- [webp](https://github.com/chai2010/webp) - WebP Image Processing
- [cron](https://github.com/robfig/cron) - Task Scheduling
- [open-golang](https://github.com/skratchdot/open-golang) - File Opening Utility
- [go-qrcode](https://github.com/skip2/go-qrcode) - QR Code Generation

## Access The Admin Backend

If you clone the code from GitHub and compile it yourself, you need to compile the admin backend code first. The admin code is at https://github.com/fesiong/anqicms-admin.

You can also download the latest release from the admin backend releases and unzip `system.zip` into the `system` folder in the project root directory.

The default backend URL is `http://127.0.0.1:8001/system`

Default admin credentials (for non-initialized installations):

Account: `admin`

Password: `123456`

## Documentation & Examples

[Demo Website](https://www.anqicms.com/manual) · [Development Docs](https://www.anqicms.com/help)

## Issue Feedback

If you encounter any problems, please open an issue on GitHub.

You can also add WeChat: `websafety` to join the Golang development learning group.

![Scan the QR Code](https://www.anqicms.com/uploads/202211/09/1a55bfcde55aa2d6.webp)

## License

AGPL-3.0 license

Copyright (c) 2019-NOW  AnQiCMS <tpyzlxy@gmail.com>
