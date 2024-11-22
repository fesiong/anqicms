<div align="center"><a name="readme-top"></a>

<img height="180" src="docs/anqicms.svg" />

# AnQiCMS: A high-performance content management system.

<div align="center">
<a href="https://github.com/fesiong/anqicms/releases"><img alt="GitHub release" src="https://img.shields.io/github/release/fesiong/anqicms.svg?style=flat-square&include_prereleases" /></a>
<a href="https://hub.docker.com/r/anqicms/anqicms"><img alt="Docker pulls" src="https://img.shields.io/docker/pulls/anqicms/anqicms?style=flat-square" /></a>
<a href="https://github.com/fesiong/anqicms/commits"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/fesiong/anqicms.svg?style=flat-square" /></a>
</div>

[Official](https://www.anqicms.com) · [Docs](https://www.anqicms.com/help) · [Demo](https://demo.anqicms.com) · [GitCode](https://gitcode.com/anqicms/anqicms) · [Changelog](./CHANGELOG.md) · English · [中文](./README.md)

</div>

## Introduce

AnQiCMS is a high-performance content management system developed using the Go programming language. It supports multi-site and multilingual management, offering flexible content publishing and template management capabilities. The system is equipped with rich SEO-friendly features, including custom fields, document categorization, and batch import/export, making it an ideal choice for building websites.

AnQiCMS aims to provide small and medium-sized enterprises with an efficient, stable, and flexible content management platform, particularly catering to businesses and individual users requiring high security, cross-site and multilingual support, and large-scale data management. Leveraging the high-performance advantages of the Go language, AnQiCMS not only meets basic content management needs but also addresses core issues such as website security, high performance, multi-site management, and multilingual support through ongoing feature expansion and performance optimization.

Traditional CMS systems face numerous challenges, including recurring legal disputes that pose potential risks to businesses. Additionally, traditional CMS solutions struggle with handling large volumes of article data, often resulting in high server resource consumption and slow page loading, with limited improvement through conventional optimization methods. Furthermore, PHP-based CMS systems are prone to security vulnerabilities, such as website defacement and hacking incidents. This has driven us to break free from the limitations of PHP and develop a more secure, flexible, and stable content management system using the Go programming language.

## Quick Start

### Quick (aaPanel) Deployment

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

[Template Documentation](https://www.AnQiCMS.com/manual)  
[Usage Guide](https://www.AnQiCMS.com/help)  
[API Documentation](https://www.AnQiCMS.com/anqiapi)

> Warm reminder: Most documents are written in Chinese. If you are an English user, please use the browser's built-in translation function to translate them yourself.

## AnQiCMS Features

- **Global Settings**: Provides unified site configuration for centralized management of all global system settings.  
- **Content Security Settings**: Includes sensitive word filtering and content review to ensure compliance.  
- **Multi-Site Management**: Supports managing multiple independent sites, ideal for multi-brand or multi-theme websites.  
- **Navigation Settings**: Configure and manage navigation bars to enhance the user navigation experience.  
- **Custom Content Models**: Allows users to define specific content models, supporting flexible content structures.  
- **Single Page Management**: Specialized tools for managing single pages like privacy policies and "About Us" pages.  
- **Pseudo-Static Rule Management**: Supports custom pseudo-static URLs to optimize SEO performance.  
- **Automatic Sitemap Generation**: Automatically generates a site sitemap to facilitate search engine crawling.  
- **Link Push Management**: Pushes new content to search engines to speed up indexing.  
- **301 Redirect Management**: Easily set up 301 redirects for URLs, improving SEO and user experience.  
- **Robots.txt Configuration**: Customize the Robots.txt file to control search engine crawling.  
- **Multi-Language Site Support**: Enables configuration and switching of multilingual sites, ideal for international websites.  
- **User Management and VIP Grouping**: Set access permissions for different user groups, suitable for membership-based websites.  
- **WeChat Official Account Integration**: Integrates with WeChat official accounts for easy content distribution and management.  
- **Site-Wide Replacement Tool**: Allows batch replacement of keywords or links for convenient content updates.  
- **Email Notifications**: Sends email alerts triggered by events such as registration or comments to boost user engagement.  
- **Keyword Library Management**: Centralized management of SEO keywords to aid in content optimization.  
- **Content Collection**: Supports automatic content collection from other websites for automated updates.  
- **Document Recycle Bin**: Allows recovery of deleted documents to prevent data loss.  
- **Time Factor - Scheduled Publishing**: Enables scheduled content publishing for automated content operations.  
- **Order Management**: Built-in order management functionality, suitable for e-commerce and service payment sites.  
- **Finance Management Module**: Records financial data and manages transaction details, ideal for subscription or transactional websites.  
- **Distribution Management**: Supports configuring distribution channels to facilitate growth through referrals.  
- **Anti-Crawling Disruption Code**: Generates disruption codes to protect content from bulk crawling.  
- **Anchor Text Management**: Allows automatic internal keyword linking to improve SEO.  
- **Watermark Management**: Adds watermarks to images to protect original copyrights.  
- **Automatic Title Images**: Automatically assigns images based on titles to enhance visual appeal.  
- **Static Page Caching**: Boosts page load speed and reduces server load.  
- **Backup and Recovery**: Supports data backup and recovery to ensure data security.  
- **Traffic and Spider Statistics**: Tracks website visits and search engine crawler activities for data analysis.  

## AnQiCMS Development History

- Latest
  > We have been working hard, exploring, iterating and optimizing to provide users with a better user experience.

- **November 11, 2024, v3.4.1 Released**  
  > **Multi-Language Sites**: Introduced multi-language site support, enabling websites to easily cater to users from different regions and cultural backgrounds, enhancing global service capabilities.  

- **September 5, 2024, v3.3.12 Released**  
  > Optimized document query efficiency, reduced resource consumption, and improved website performance, particularly for scenarios with a large volume of articles.  

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
[demo - https://www.AnQiCMS.com/](https://www.AnQiCMS.com/manual)


## 👥 Issue
If you encounter any problems, please open an issue on Github.
You can also add my WeChat: websafety

Scan the QR code to join the golang development learning group

![Scan the QR Code](https://www.AnQiCMS.com/uploads/202211/09/1a55bfcde55aa2d6.webp)

## License
AGPL-3.0 license

Copyright (c) 2019-NOW  AnQiCMS <tpyzlxy@gmail.com>
