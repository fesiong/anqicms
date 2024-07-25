<div align="center"><a name="readme-top"></a>

<img height="180" src="docs/AnQiCMS.svg" />

[![downloads][download-image]][download-url] [![FOSSA Status][fossa-image]][fossa-url] [![Issues need help][help-wanted-image]][help-wanted-url]

[Changelog](./CHANGELOG.md) · [Report Bug][github-issues-url] · [Request Feature][github-issues-url] · English · [中文](./README-zh_CN.md)

# AnQiCMS

</div>

## Introduce

The predecessor of AnQi Content Management System (AnQiCMS) is GoBlog, a simple personal blog system based on the iris framework and developed using golang

> GoBlog is an open source personal blog system written in golang, with an elegant interface, compact, fast execution speed, and SEO-friendly, which can meet daily blog needs. It is very simple to use and deploy, and is adaptive to PC and mobile terminals. The page template uses a syntax similar to the blade template engine, which is very easy to use and suitable for personal blogs.

Anqi Content Management System (AnQiCMS) is an enterprise content management system developed with GoLang based on the iris framework. It is easy to deploy, and the software is more secure than the traditional PHP-developed content management system. It has an elegant, compact interface and fast execution speed. Websites built with AnQiCMS can prevent many common security issues. AnQiCMS is designed to be SEO-friendly, and has a large number of common functions for enterprise sites built in, which is very helpful for website optimization. It improves the efficiency of enterprise management websites to a certain extent and improves the competitiveness of enterprises.

In addition to being suitable for enterprise sites, AnQiCMS is also suitable for marketing websites, corporate official websites, product display sites, government websites, portals, personal blogs and other types of websites.

AnQiCMS supports Django template engine syntax, which is similar to blade syntax, and can be very easy to get started with template making. The website mode supports three modes: adaptive, code adaptation, and PC+mobile independent site. According to different needs, you can choose the combination that suits you to build a website.

Our pursuit: Make the world a safe website.

We have been moving towards website security. Let AnQiCMS protect your website security.

Welcome to use AnQiCMS.

## Quick Start

[Download AnQiCMS](https://github.com/fesiong/goblog/releases)
[Installation Guide](https://www.AnQiCMS.com/help-basic/210.html)  
[Template Documentation](https://www.AnQiCMS.com/manual)  
[Usage Guide](https://www.AnQiCMS.com/help)  
[API Documentation](https://www.AnQiCMS.com/anqiapi)

> Warm reminder: Most documents are written in Chinese. If you are an English user, please use the browser's built-in translation function to translate them yourself.

## AnQiCMS Features

- Customized document model
- Customized page navigation
- Rich text, Markdown editor support
- Webp image support
- Multiple template customization support
- Multiple site support
- Detailed data statistics record
- Customized pseudo-static rules
- Active push by multiple search engines
- Sitemap management
- Robots.txt management
- Friendly link management
- Content comment management
- Automatic anchor text function
- Website message management
- Keyword library management
- Content material management
- Email reminder function
- Article collection function
- Article combination function
- Article import function
- Customized 301 jump function
- Website content migration function
- Static page function
- Customized resource storage
- User management
- User group management
- Mini program support
- Full-text search support
- Backup and recovery
- Article automatic picture support
- AI automatic writing function
- Scheduled release/update function
- Anti-collection interference function
- Image watermark function

## AnQiCMS Development History

- Latest
  > We have been working hard, exploring, iterating and optimizing to provide users with a better user experience.
- May 1, 2024, v3.3.5 released
  > Support image watermark function
- October 24, 2023, v3.2.5 released
  > Support multi-language translation function, support Markdown editor
- April 15, 2023, v3.1.1 released
  > Access AI automatic writing function
- December 5, 2022, v3.0.0 released
  > Started to support multi-site mode, simplified and reduced the deployment difficulty of AnQiCMS, and added more common functions for enterprise sites
- May 30, 2022, v2.1.0 released
  > Officially renamed AnQiCMS, indicating that AnQiCMS has the necessary functions of common content management systems
- February 16, 2021, v2.0.0-alpha released
  > We started to gradually transition from a simple blog function to a more comprehensive content management system, and gradually developed and improved the enterprise site functions.
- January 21, 2021, GoBlog v1.0.0 released
  > The perfect blog version, based on the basic version, added background management, SEO functions, etc.
- December 1, 2020, GoBlog v0.5 released
  > The refactored version was rewritten using the iris framework, reducing the technology stack and using the template engine that comes with iris. The most basic blog functions were realized.
- November 19, 2019, GoBlog v0.1 released
  > The Gin version, with front-end and back-end separation, uses go, gin, gorm for the back-end, and Next.js for the front-end.

## Packages Used

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

## Access The Admin Backend
If you clone the downloaded code from GitHub and compile and run it yourself, you need to compile the backend management code first. The backend management code is at https://github.com/fesiong/AnQiCMS-admin.
You can also download the latest release from the release of the backend management code and unzip system.zip to the system folder in the project root directory.

The default backend address is http://127.0.0.1:8001/system

If you did not initialize through installation, you may not have set up an administrator account. If you do not set up an administrator account, the default administrator account password is:

Account: admin

Password: 123456

## Sample Website & Development Documentation
[示例网站 - https://www.AnQiCMS.com/](https://www.AnQiCMS.com/manual)


## 👥 Issue
If you encounter any problems, please open an issue on Github.
You can also add my WeChat: websafety

Scan the QR code to join the golang development learning group

![Scan the QR Code](https://www.AnQiCMS.com/uploads/202211/09/1a55bfcde55aa2d6.webp)

## License
AnQiCMS End User License Agreement

Copyright (c) 2019-NOW  Fesion <tpyzlxy@gmail.com>
