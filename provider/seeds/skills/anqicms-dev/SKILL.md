---
name: anqicms-dev
description: AnQiCMS 开发核心技能：模板语法规则、引擎说明（pongo2）、标签闭合规则、API 规范、数据模型
category: Development
version: 1.2
tags: [anqicms, development, template, api, pongo2]
---

# AnQiCMS 开发核心技能

## 模板引擎

AnQiCMS 使用基于 `github.com/flosch/pongo2` 的 Django-like 模板引擎，语法与 Django/Jinja2 类似但有自定义扩充。

## 模板语法核心规则

| 规则 | 说明 | 正确示例 | 错误示例 |
|---|---|---|---|
| 变量大小写 | **严格区分大小写**，字段首字母大写 | `{{ item.Title }}` | `{{ item.title }}` |
| 时间格式 | 使用 Golang 参考时间（不是 PHP/Python 格式） | `{{ stampToDate(item.CreatedTime, "2006-01-02") }}` | `{{ item.CreatedTime\|date("Y-m-d") }}` |
| 标签闭合 | 只有 BLOCK_TAGS 需闭合，SINGLE_TAGS 无需闭合 | `{% archiveList %}...{% endarchiveList %}` | 对 `{% tdk %}` 加 `{% endtdk %}` |
| 条件判断 | 用 `elif` 不是 `elseif` | `{% elif cond %}` | `{% elseif cond %}` |
| 空值判断 | 可用 `{% if array %}` 或 `{% for %}...{% empty %}...{% endfor %}` | | |
| 变量声明 | 获取数据时需声明变量名才能后续引用 | `{% contact siteInfo %}` | 不声明直接 `{% if contact %}` |

## 标签闭合规则速查

### 需闭合（BLOCK_TAGS，32 个 = 20 自定义 + 12 pongo2 内置）

`archiveList` · `categoryList` · `navList` · `pagination` · `tagList` · `bannerList` · `commentList` · `guestbookList` · `placeList` · `if` · `for` · `block` · `with` · `autoescape` · `filter` · `ifchanged` · `ifequal` · `ifnotequal` · `macro` · `spaceless` · `pageList` · `prevArchive` · `nextArchive` · `breadcrumb` · `linkList` · `guestbook` · `archiveParams` · `tagDataList` · `archiveFilters` · `languages`（闭合 `endLanguages`）· `jsonLd` · `comment`

### 自闭合（SINGLE_TAGS，26 个 = 15 自定义 + 11 pongo2 内置，不需要 end 前缀）

`archiveDetail` · `categoryDetail` · `tagDetail` · `pageDetail` · `moduleDetail` · `placeDetail` · `userDetail` · `userGroupDetail` · `tdk` · `system` · `contact` · `diy` · `tr` · `lorem` · `now` · `set` · `include` · `extends` · `import` · `ssi` · `templatetag` · `widthratio` · `attachment` · `cycle` · `firstof` · `jump`

完整说明请加载 `template-dev` 技能。

## API 核心规范

AnQiCMS 有两套 API：
- **前端公共 API**：前缀 `/api`（如 `https://domain.com/api/archive/list`），Header `token: <jwt>`，滑动轮换的新 token 在响应头 `update-token`。详见 `api-dev` 技能。
- **后台管理 API**：前缀 `/system/api`，Header `Admin: <token>`。AI/MCP 意图工具走的就是这一套。

- **数据格式**：JSON（文件上传为 multipart）
- **分页**：内容列表用 `page` + `limit`；用户/订单/分销列表用 `current` + `pageSize`（默认 20）。后台管理列表统一 `current` + `pageSize`。

### 响应格式

```typescript
interface ApiResponse<T> {
  code: number;      // 0=成功；-1=失败；1001=未登录/token失效；1002=无权限
  msg: string;       // 错误消息
  data: T;           // 数据
  total?: number;    // 分页总数（仅列表接口）
}
```

**关键规则**：检查 `code === 0`，HTTP 200 不代表成功。`code === 1001` 表示 token 过期需重新登录（码值定义见 `config/constant.go`）。

### 核心数据模型

JSON 字段名（API 输出用）；模板里访问则用**首字母大写的 Go 字段名**（如 `{{ item.Title }}`）。

| 模型 | 关键字段（json） |
|---|---|
| **Archive**（文章/产品） | `id`, `title`, `seo_title`, `description`, `keywords`, `created_time`/`updated_time`(timestamp), `views`, `comment_count`, `favorite_count`, `category_id`, `module_id`, `place_id`, `url_token`, `images`, `logo`, `thumb`, `link`, `tags`, `price`, `stock`, `read_level`, `sort`, `template`, `canonical_url`, `fixed_link`；正文在 `data`(ArchiveData) 里。**没有 `cover` 字段**，封面用 `logo`/`images`。 |
| **Category**（分类） | `id`, `title`, `parent_id`, `logo`, `images`, `link`, `thumb`, `type`（1=文档分类，3=单页，默认 0）, `children`, `archive_count`, `status` |
| **Order**（订单） | `id`, `order_id`(字符串单号), `amount`/`origin_amount`(int64，分为单位), `user_id`, `payment_id`, `status`, `refund_status`, `details` |
| **User**（用户） | `id`, `user_name`, `avatar_url`, `balance`, `phone`, `email`, `group_id`, `status`（1 正常/0 待审/-1 封禁）；`password` 不输出 |

**Order.status 取值**（`config/constant.go`）：`-1` 取消/关闭 · `0` 待付款 · `1` 已付款 · `2` 发货中 · `3` 已完成 · `8` 退款中 · `9` 已退款。

## 模板目录结构

```
template/your_template_name/
├── base.html              # 基础布局（header/footer）
├── index/
│   └── index.html         # 首页
├── article/               # 文章模型
│   ├── index.html         # 文章首页（模型页）
│   ├── list.html          # 文章列表
│   └── detail.html        # 文章详情
├── product/               # 产品模型（有产品模型时）
│   ├── index.html
│   ├── list.html
│   └── detail.html
├── page/
│   ├── detail.html        # 单页面
│   └── about.html         # 自定义别名
├── search/
│   └── index.html         # 搜索结果
├── tag/
│   ├── index.html         # 标签云
│   └── list.html          # 标签文档列表
├── comment/
│   └── list.html          # 评论
├── guestbook/
│   └── index.html         # 留言板
├── partial/               # 可复用片段
│   ├── header.html
│   ├── footer.html
│   └── sidebar.html
└── errors/
    ├── 404.html
    ├── 500.html
    └── close.html         # 站点关闭
```

### 自定义覆盖规则
- **文档详情**：`{模型表名}/{文档ID}.html`（如 `article/10.html`）
- **文档列表**：`{模型表名}/list-{分类ID或别名}.html`（如 `article/list-5.html`）
- **单页面**：`page/{别名}.html`（如 `page/about.html`）

## 静态资源引用

CSS/JS/图片放在 `/public/static/{your_template_name}/` 目录下，模板中通过 `system` 标签引用路径：

```twig
<link href="{% system with name='TemplateUrl' %}css/style.css" rel="stylesheet">
<script src="{% system with name='TemplateUrl' %}js/main.js"></script>
```

## 操作流程

> AI 工具面是**意图工具**：调用时传意图名 + `action`。模板重载、技能加载分别属于 `system_config` 与 `skill` 意图。

1. 修改模板文件 → 调用 `system_config`（action=`template_reload`）生效
2. 修改 CSS → 清除浏览器缓存后刷新
3. 新增模板 → 创建对应目录和文件，确保目录结构正确
4. 调用 `skill`（action=`get`, name=`template-dev`）获取模板标签完整参考
5. 调用 `skill`（action=`get`, name=`api-dev`）获取完整的前端 API 端点列表
