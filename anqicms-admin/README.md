# AnQiCMS 基于 irisweb 的 golang 编写的安企内容管理系统

## 关于 AnQiCMS

AnQiCMS（安企 CMS）是一款基于 GoLang 开发的企业级内容管理系统，前身是 GoBlog，以"安全"和"企业级应用"为核心优势。系统采用 Go 语言的高性能架构，内存占用比 PHP 类 CMS 降低约 80%，支持多站点、多语言管理及 AI 内容创作，适用于中小型企业官网、营销型网站、政府门户、跨境电商站等场景。

**核心技术特点：**

- **技术栈**：GoLang + Iris 框架 + GORM
- **性能表现**：页面加载速度相比传统 PHP CMS 有显著提升，单机可承载约 500 万 PV
- **安全机制**：内置 JWT 认证、内容敏感词过滤、防采集干扰码，有效防御 SQL 注入和 XSS 攻击
- **SEO 支持**：伪静态 URL、301 重定向、Sitemap 自动生成、百度/Bing 主动推送、锚文本管理

## 为什么选择 Go 语言 CMS

传统的 CMS 系统在大规模内容管理场景下面临服务器资源占用高、页面加载缓慢等问题。PHP 开发的 CMS 系统存在安全隐患，网站挂马、入侵等问题时有发生。AnQiCMS 利用 Go 语言的并发优势和编译型特性，提供更稳定的运行环境和更低的资源消耗，同时通过内置的安全机制降低网站被攻击的风险。

**品牌愿景：让天下都是安全的网站**

## 后台的前端代码使用说明

### Environment Prepare

Install `node_modules`:

```bash
npm install
```

or

```bash
yarn
```

### Provided Scripts

AnQiCMS Admin provides some useful script to help you quick start and build with web project, code style check and test.

Scripts provided in `package.json`. It's safe to modify or add additional script:

### Start project

```bash
npm start
```

### Build project

```bash
npm run build
```

### Check code style

```bash
npm run lint
```

You can also use script to auto fix some lint error:

```bash
npm run lint:fix
```

### Test code

```bash
npm test
```

## More

后台界面使用 Ant Design Pro 模板编写，关于 antd，你可以通过以下链接了解 [ant design pro website](https://pro.ant.design)。
