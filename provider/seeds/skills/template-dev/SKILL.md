---
name: template-dev
description: AnQiCMS 模板开发技能：基于 pongo2（Django 语法）的自定义模板引擎，标签闭合规则完整速查
category: Development
version: 1.1
tags: [anqicms, template, pongo2, django, tag]
---

# AnQiCMS 模板开发技能

AnQiCMS 使用基于 `github.com/flosch/pongo2`（Django-like syntax）的模板引擎，并在此基础上做了自定义扩充。

## 核心语法

- **变量**：`{{ item.Title }}` — **严格区分大小写**，字段首字母大写
- **过滤器**：`{{ var|filter }}`
- **注释**：`{# 单行 #}` 或 `{% comment %}...{% endcomment %}`
- **时间格式**：使用 Golang 参考时间 `2006-01-02 15:04:05`
  - 函数：`{{ stampToDate(timestamp, "2006-01-02") }}`
  - 过滤器：`{{ timestamp|dateFormat:"2006-01-02" }}`
- **空值判断**：`{% if array %}` 或 `{% for %}...{% empty %}...{% endfor %}`
- **条件**：`{% if cond %}...{% elif %}...{% else %}...{% endif %}`（用 `elif` 不是 `elseif`）
- **变量声明**：用标签获取数据时需声明变量名才能后续引用

## 标签闭合规则

> **并非所有标签都需要闭合。** 以下按类型区分。

### 需闭合标签（BLOCK_TAGS）
这些标签必须成对使用：`{% tag %}...{% endtag %}``

| 标签 | 闭合 | 用途 |
|---|---|---|
| `archiveList` | `endarchiveList` | 文档列表 |
| `categoryList` | `endcategoryList` | 分类列表 |
| `navList` | `endnavList` | 导航 |
| `pagination` | `endpagination` | 分页 |
| `tagList` | `endtagList` | 标签列表 |
| `bannerList` | `endbannerList` | 横幅 |
| `commentList` | `endcommentList` | 评论列表 |
| `reviewList` | `endreviewList` | 评价列表 |
| `if` | `endif` | 条件判断 |
| `for` | `endfor` | 循环 |
| `block` | `endblock` | 模板块 |
| `with` | `endwith` | 局部变量 |
| `autoescape` | `endautoescape` | 转义控制 |
| `filter` | `endfilter` | 过滤器块 |
| `ifchanged` | `endifchanged` | 分组变化 |
| `ifequal` | `endifequal` | 相等判断 |
| `ifnotequal` | `endifnotequal` | 不等判断 |
| `macro` | `endmacro` | 宏 |
| `spaceless` | `endspaceless` | 空白压缩 |
| `pageList` | `endpageList` | 页面列表 |
| `prevArchive` | `endprevArchive` | 上一篇 |
| `nextArchive` | `endnextArchive` | 下一篇 |
| `breadcrumb` | `endbreadcrumb` | 面包屑 |
| `linkList` | `endlinkList` | 友情链接 |
| `guestbook` | `endguestbook` | 留言板 |
| `archiveParams` | `endarchiveParams` | 文档参数 |
| `tagDataList` | `endtagDataList` | 标签文档列表 |
| `archiveFilters` | `endarchiveFilters` | 筛选器 |
| `languages` | `endLanguages` | 多语言切换 |
| `jsonLd` | `endjsonLd` | 结构化数据 |
| `comment` | `endcomment` | 多行注释 |
| `archiveSku` | `endarchiveSku` | 产品SKU |

### 自闭合标签（SINGLE_TAGS）
这些标签**不需要**闭合，单行使用即可：

`archiveDetail` · `categoryDetail` · `tagDetail` · `pageDetail` · `moduleDetail` ·
`userDetail` · `userGroupDetail` · `tdk` · `system` · `contact` · `diy` ·
`pluginJsCode` · `tr` · `lorem` · `now` · `set` · `include` · `extends` · `import` ·
`ssi` · `templatetag` · `widthratio` · `attachment` · `cycle` · `firstof` · `jump`

```twig
{% tdk t %}
{% system siteInfo %}
{% contact contactInfo %}
{% archiveDetail article with id=123 %}
{% include "partial/header.html" %}
{% extends "base.html" %}
{% set varName = value %}
```

### 控制标签（CONTROL_TAGS）
这些标签必须出现在特定的块标签内部：

| 标签 | 必须位于 | 说明 |
|---|---|---|
| `{% else %}` | `{% if %}...{% endif %}` 内部 | 条件分支 |
| `{% elif %}` | `{% if %}...{% endif %}` 内部 | 多条件 |
| `{% empty %}` | `{% for %}...{% endfor %}` 内部 | 空数据提示 |

## 常用标签速查

### 内容获取
```twig
{% archiveList articles with moduleId=1 categoryId=2 limit="10" order="id desc" %}
  {% for article in articles %}
    <h3><a href="{{ article.Link }}">{{ article.Title }}</a></h3>
    <span>{{ article.CreatedTime|dateFormat:"2006-01-02" }}</span>
  {% empty %}
    <p>暂无内容</p>
  {% endfor %}
{% endarchiveList %}
```

| 标签 | 关键参数 |
|---|---|
| `archiveList` | moduleId(1=文章,2=产品), categoryId, flag(c/h/f), limit, order, type |
| `archiveDetail` | id 或 url_token |
| `categoryList` | type=list/tree, moduleId |
| `tagDataList` | tagId |
| `archiveFilters` | moduleId, allText |

### 页面组件
```twig
{% pagination pages with show="5" %}
  {% if pages.PrevPage %}<a href="{{ pages.PrevPage.Link }}">上一页</a>{% endif %}
  {% for page in pages.Pages %}
    <a href="{{ page.Link }}" class="{% if page.IsCurrent %}active{% endif %}">{{ page.Name }}</a>
  {% endfor %}
  {% if pages.NextPage %}<a href="{{ pages.NextPage.Link }}">下一页</a>{% endif %}
{% endpagination %}

{% navList navs with type="top" %}
  {% for nav in navs %}
    <a href="{{ nav.Link }}" {% if nav.IsCurrent %}class="active"{% endif %}>{{ nav.Title }}</a>
  {% endfor %}
{% endnavList %}

{% breadcrumb crumbs %}
  {% for crumb in crumbs %}
    <a href="{{ crumb.Link }}">{{ crumb.Name }}</a>
  {% endfor %}
{% endbreadcrumb %}
```

### 逻辑控制
```twig
{% if user %}
  {{ user.UserName }}
{% elif guest %}
  访客
{% else %}
  未知
{% endif %}

{% for item in items %}
  {{ item.Name }}
{% empty %}
  列表为空
{% endfor %}
```

## 过滤器与函数

**时间**：`{{ stampToDate(timestamp, "2006-01-02") }}` · `{{ timestamp|dateFormat:"2006-01-02" }}`

**价格**：`{{ priceFormat(price) }}` · `{{ price|priceFormat }}`

**标准过滤器**：`default` · `length` · `upper` · `lower` · `title` · `trim` · `urlencode` · `addslashes` · `slugify` · `split(",")` · `first` · `last` · `cut` · `replace` · `center` · `wordcount` · `random`

## 验证清单

- [ ] 字段首字母大写（`Title` 不是 `title`，`Link` 不是 `link`）
- [ ] 需闭合的标签正确成对（见上方 BLOCK_TAGS 表）
- [ ] 自闭合标签没有错误添加 `end` 前缀
- [ ] `elif` 不是 `elseif`
- [ ] 时间用 `stampToDate` 函数或 `dateFormat` 过滤器
- [ ] 模板目录结构遵循标准布局（`base.html` + 模型目录）
- [ ] 修改后运行 `template_reload` 生效
