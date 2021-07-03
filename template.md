# 模板标签说明
注意：标签中，严格区分大小写，如果大小写拼写不正确，结果也会不对的。
更详细的模板标签使用教程，请查看[模板标签手册](https://www.kandaoni.com/category/10)

## 系统标签
说明：用于获取系统配置信息

使用方法：`{% system 变量名称 with name="字段名称" %}`，变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。name 参数可用的字段名称有：

- 网站名称 `SiteName`
- 网站Logo `SiteLogo`
- 网站备案号 `SiteIcp`
- 版权内容 `SiteCopyright`
- 网站首页地址 `BaseUrl`
- 网站首页地址 `BaseUrl`
- 网站手机端地址 `MobileUrl`

### 网站名称 `SiteName`
标签用法：`{% system with name="SiteName" %}`
```twig
{# 默认用法 #}
<div>网站名称：{% system with name="SiteName" %}</div>
{# 自定义名称调用 #}
<div>网站名称：{% system siteName with name="SiteName" %}{{siteName}}</div>
```

### 网站Logo `SiteLogo`
标签用法：`{% system with name="SiteLogo" %}`
```twig
{# 默认用法 #}
<img src="{% system with name="SiteLogo" %}" alt="{% system with name="SiteName" %}" />
{# 自定义名称调用 #}
<img src="{% system siteLogo with name="SiteLogo" %}{{siteLogo}}" alt="{% system siteName with name="SiteName" %}{{siteName}}" />
```

### 网站备案号 `SiteIcp`
标签用法：`{% system with name="SiteIcp" %}`
```twig
{# 默认用法 #}
<p><a href="https://beian.miit.gov.cn/" rel="nofollow" target="_blank">{% system with name="SiteIcp" %}</a> &copy;2021 kandaoni.com. All Rights Reserved</p>
{# 自定义名称调用 #}
<p><a href="https://beian.miit.gov.cn/" rel="nofollow" target="_blank">{% system siteIcp with name="SiteIcp" %}{{siteIcp}}</a> &copy;2021 kandaoni.com. All Rights Reserved</p>
```

### 版权内容 `SiteCopyright`
标签用法：`{% system with name="SiteCopyright" %}`
```twig
{# 默认用法 #}
<div class="layout">{% system with name="SiteCopyright" %}</div>
{# 自定义名称调用 #}
<div class="layout">{% system siteCopyright with name="SiteCopyright" %}{{siteCopyright|safe}}</div>
```

### 网站首页地址 `BaseUrl`
标签用法：`{% system with name="BaseUrl" %}`
```twig
{# 默认用法 #}
<div>首页地址：{% system with name="BaseUrl" %}</div>
{# 自定义名称调用 #}
<div>首页地址：{% system baseUrl with name="BaseUrl" %}{{baseUrl|safe}}</div>
```
### 网站手机端地址 `MobileUrl`
标签用法：`{% system with name="MobileUrl" %}`
```twig
{# 默认用法 #}
<div>手机地址：{% system with name="MobileUrl" %}</div>
{# 自定义名称调用 #}
<div>手机地址：{% system mobileUrl with name="MobileUrl" %}{{mobileUrl|safe}}</div>
```

## 联系方式标签
说明：用于获取后台配置的联系方式信息

使用方法：`{% contact 变量名称 with name="字段名称" %}`，变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。name 参数可用的字段名称有：

- 联系人 `UserName`
- 联系电话 `Cellphone`
- 联系地址 `Address`
- 联系邮箱 `Email`
- 联系微信 `Wechat`
- 微信二维码 `Qrcode`

### 联系人 `UserName`
标签用法：`{% contact with name="UserName" %}`
```twig
{# 默认用法 #}
<li class="item contact-item">
    <span class="item-left">联系人</span>
    <span class="item-right">{% contact with name="UserName" %}</span>
</li>
{# 自定义名称调用 #}
<li class="item contact-item">
    <span class="item-left">联系人</span>
    <span class="item-right">{% contact userName with name="UserName" %}{{userName}}</span>
</li>
```

### 联系电话 `Cellphone`
标签用法：`{% contact with name="Cellphone" %}`
```twig
{# 默认用法 #}
<li class="item contact-item">
    <span class="item-left">联系电话</span>
    <span class="item-right">{% contact with name="Cellphone" %}</span>
</li>
{# 自定义名称调用 #}
<li class="item contact-item">
    <span class="item-left">联系电话</span>
    <span class="item-right">{% contact cellphone with name="Cellphone" %}{{cellphone}}</span>
</li>
```

### 联系地址 `Address`
标签用法：`{% contact with name="Address" %}`
```twig
{# 默认用法 #}
<li class="item contact-item">
    <span class="item-left">联系地址</span>
    <span class="item-right">{% contact with name="Address" %}</span>
</li>
{# 自定义名称调用 #}
<li class="item contact-item">
    <span class="item-left">联系地址</span>
    <span class="item-right">{% contact address with name="Address" %}{{address}}</span>
</li>
```

### 联系邮箱 `Email`
标签用法：`{% contact with name="Email" %}`
```twig
{# 默认用法 #}
<li class="item contact-item">
    <span class="item-left">联系邮箱</span>
    <span class="item-right">{% contact with name="Email" %}</span>
</li>
{# 自定义名称调用 #}
<li class="item contact-item">
    <span class="item-left">联系邮箱</span>
    <span class="item-right">{% contact contactEmail with name="Email" %}{{contactEmail}}</span>
</li>
```

### 联系微信 `Wechat`
标签用法：`{% contact with name="Wechat" %}`
```twig
{# 默认用法 #}
<li class="item contact-item">
    <span class="item-left">联系微信</span>
    <span class="item-right">{% contact with name="Wechat" %}</span>
</li>
{# 自定义名称调用 #}
<li class="item contact-item">
    <span class="item-left">联系微信</span>
    <span class="item-right">{% contact contactWechat with name="Wechat" %}{{contactWechat}}</span>
</li>
```

### 微信二维码 `Qrcode`
标签用法：`{% contact with name="Qrcode" %}`
```twig
{# 默认用法 #}
<li class="item contact-item">
    <span class="item-left">微信二维码</span>
    <span class="item-right">
        <img src="{% contact with name="Qrcode" %}" style="max-width: 200px;" />
    </span>
</li>
{# 自定义名称调用 #}
<li class="item contact-item">
    <span class="item-left">微信二维码</span>
    <span class="item-right">
        <img src="{% contact qrcode with name="Qrcode" %}{{qrcode|safe}}" style="max-width: 200px;" />
    </span>
</li>
```

## 万能TDK标签
说明：用于获取页面的title、keywords、description信息

使用方法：`{% tdk 变量名称 with name="字段名称" %}`，变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。name 参数可用的字段名称有：

- 页面标题 `Title`
- 页面关键词 `Keywords`
- 页面描述 `Description`

### 页面标题 `Title`
标签用法：`{% tdk with name="Title" %}` name="Title" 时变量支持 `siteName` 属性，可以设置是否附加网站名称到Title后面。 `siteName` 为一个bool类型，默认不显示。显示的时候使用： `{% tdk with name="Title" siteName=true %}`
```twig
{# 不显示网站名称后缀 #}
<title>{% tdk with name="Title" %}</title>
{# 显示网站名称后缀 #}
<title>{% tdk with name="Title" siteName=true %}</title>
{# 不显示网站名称后缀 #}
<title>{% tdk with name="Title" siteName=false %}</title>
{# 自定义名称调用 #}
<title>{% tdk seoTitle with name="Title" siteName=true %}{{seoTitle}}</title>
```
### 页面关键词 `Keywords`
标签用法：`{% tdk with name="Keywords" %}`
```twig
{# 默认用法 #}
<meta name="keywords" content="{% tdk with name="Keywords" %}">
{# 自定义名称调用 #}
<meta name="keywords" content="{% tdk seoKeywords with name="Keywords" %}{{seoKeywords}}">
```
### 页面描述 `Description`
标签用法：`{% tdk with name="Description" %}`
```twig
{# 默认用法 #}
<meta name="description" content="{% tdk with name="Description" %}">
{# 自定义名称调用 #}
<meta name="description" content="{% tdk seoDescription with name="Description" %}{{seoDescription}}">
```

## 导航列表标签
说明：用于获取页面导航列表

使用方法： `{% navList 变量名称 %}` 如将变量定义为navs `{% navList navs %}...{% endnavList %}`，也可以定义为其他变量名称，定义后，需要与下面的for循环使用的变量名称一致。 navList 标签没有参数，需要使用使用 endnavList 标签表示结束，中间使用for循环输出内容。

item 为for循环体内的变量，可用的字段有：

- 导航标题 `Title`
- 子标题 `SubTitle`
- 导航描述 `Description`
- 导航链接 `Link`
- 是否当前链接 `IsCurrent`
- 下级导航列表 `NavList` 下级导航内同样具有 item 相同的字段。

```twig
{% navList navs %}
<ul class="layui-nav layui-layout-left nav-list">
    {%- for item in navs %}
        <li class="layui-nav-item{% if item.IsCurrent %} layui-this{% endif %}">
            <a href="{{ item.Link }}">{{item.Title}}</a>
            {%- if item.NavList %}
            <dl class="layui-nav-child">
                {%- for inner in item.NavList %}
                    <dd class="{% if inner.IsCurrent %} layui-this{% endif %}">
                        <a href="{{ inner.Link }}">{{inner.Title}}</a>
                    </dd>
                {% endfor %}
            </dl>
            {% endif %}
        </li>
    {% endfor %}
</ul>
{% endnavList %}
```

## 分类列表标签
说明：用于获取文章、产品分类列表

使用方法：`{% categoryList 变量名称 with type="1|2" parentId="0" %}` 如将变量定义为 categories `{% categoryList categories with type="1" parentId="0" %}...{% endcategoryList %}` categoryList 支持的参数有： `type` 、 `parentId` type 值只能是1或2，type="1"时，表示获取文章分类，type="2"时，表示获取产品分类；parentId 表示上级分类，可以获取指定上级分类下的子分类，parentId="0"的时候，获取所有分类。

item 为for循环体内的变量，可用的字段有：

- 分类标题 `Title`
- 分类链接 `Link`
- 分类描述 `Description`
- 分类内容 `Content`
- 上级分类ID `ParentId`
- 下级分类前缀 `Spacer`
- 是否有下级分类 `HasChildren`

```twig
{% categoryList categories with type="1" parentId="0" %}
<ul>
    {% for item in categories %}
    <li>
        <a href="{{ item.Link }}">{{item.Spacer|safe}}{{item.Title}}</a>
        <a href="{{ item.Link }}">
            <span>分类名称：{{item.Title}}</span>
            <span>分类链接：{{item.Link}}</span>
            <span>分类描述：{{item.Description}}</span>
            <span>分类内容：{{item.Content|safe}}</span>
            <span>上级分类ID：{{item.ParentId}}</span>
            <span>下级分类前缀：{{item.Spacer|safe}}</span>
            <span>是否有下级分类：{{item.HasChildren}}</span>
        </a>
    </li>
    {% endfor %}
</ul>
{% endcategoryList %}
```

## 分类详情标签
说明：用于获取文章分类、产品分类详情

使用方法：`{% categoryDetail with name="变量名称" id="1" %}` 变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。categoryDetail 支持的参数有： `id`，id 不是必须的，默认会获取当前分类。如果需要指定分类，可以通过设置id来达到目的。

name 参数可用的字段有：

- 分类标题 `Title`
- 分类链接 `Link`
- 分类描述 `Description`
- 分类内容 `Content`
- 上级分类ID `ParentId`
- 分类ID `Id`

### 分类标题 `Title`
标签用法：`{% categoryDetail with name="Title" %}`
```twig
{# 默认用法，自动获取当前页面分类 #}
<div>分类标题：{% categoryDetail with name="Title" %}</div>
{# 获取指定分类id的分类字段 #}
<div>分类标题：{% categoryDetail with name="Title" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类标题：{% categoryDetail categoryTitle with name="Title" %}{{categoryTitle}}</div>
<div>分类标题：{% categoryDetail categoryTitle with name="Title" id="1" %}{{categoryTitle}}</div>
```

### 分类链接 `Link`
标签用法：`{% categoryDetail with name="Link" %}`
```twig
{# 默认用法，自动获取当前页面分类 #}
<div>分类链接：{% categoryDetail with name="Link" %}</div>
{# 获取指定分类id的分类字段 #}
<div>分类链接：{% categoryDetail with name="Link" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类链接：{% categoryDetail categoryLink with name="Link" %}{{categoryLink}}</div>
<div>分类链接：{% categoryDetail categoryLink with name="Link" id="1" %}{{categoryLink}}</div>
```

### 分类描述 `Description`
标签用法：`{% categoryDetail with name="Description" %}`
```twig
{# 默认用法，自动获取当前页面分类 #}
<div>分类描述：{% categoryDetail with name="Description" %}</div>
{# 获取指定分类id的分类字段 #}
<div>分类描述：{% categoryDetail with name="Description" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类描述：{% categoryDetail categoryDescription with name="Description" %}{{categoryDescription}}</div>
<div>分类描述：{% categoryDetail categoryDescription with name="Description" id="1" %}{{categoryDescription}}</div>
```

### 分类内容 `Content`
标签用法：`{% categoryDetail with name="Content" %}`
```twig
{# 默认用法，自动获取当前页面分类 #}
<div>分类内容：{% categoryDetail with name="Content" %}</div>
{# 获取指定分类id的分类字段 #}
<div>分类内容：{% categoryDetail with name="Content" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类内容：{% categoryDetail categoryContent with name="Content" %}{{categoryContent|safe}}</div>
<div>分类内容：{% categoryDetail categoryContent with name="Content" id="1" %}{{categoryContent|safe}}</div>
```

### 上级分类ID `ParentId`
标签用法：`{% categoryDetail with name="ParentId" %}`
```twig
{# 默认用法，自动获取当前页面分类 #}
<div>上级分类ID：{% categoryDetail with name="ParentId" %}</div>
{# 获取指定分类id的分类字段 #}
<div>上级分类ID：{% categoryDetail with name="ParentId" id="1" %}</div>
{# 自定义字段名称 #}
<div>上级分类ID：{% categoryDetail categoryParentId with name="ParentId" %}{{categoryParentId}}</div>
<div>上级分类ID：{% categoryDetail categoryParentId with name="ParentId" id="1" %}{{categoryParentId}}</div>
```

### 分类ID `Id`
标签用法：`{% categoryDetail with name="Id" %}`
```twig
{# 默认用法，自动获取当前页面分类 #}
<div>分类ID：{% categoryDetail with name="Id" %}</div>
{# 获取指定分类id的分类字段 #}
<div>分类ID：{% categoryDetail with name="Id" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类ID：{% categoryDetail categoryId with name="Id" %}{{categoryId}}</div>
<div>分类ID：{% categoryDetail categoryId with name="Id" id="1" %}{{categoryId}}</div>
```

## 文章列表标签
说明：用于获取文章常规列表、相关文章列表、文章分页列表

使用方法：`{% articleList 变量名称 with categoryId="1" order="id desc|views desc" type="page|list" q="搜索关键词" %}` 如将变量定义为 articles `{% articleList articles with type="page" %}...{% endarticleList %}` articleList 支持的参数有： `categoryId`，可以获取指定分类的文章列表如 `categoryId="1"` 获取文章分类ID为1的文章列表；`order` 可以指定文章显示的排序规则，支持依据 最新文章排序 `order="id desc"`、浏览量最多文章排序 `order="views desc"`；显示数量 `limit`数量的列表，比如`limit="10"`则只会显示10条； 列表类型 `type`，支持按 page、list、related 方式列出。默认值为list，`type="list"` 时，只会显示 指定的 limit 指定的数量，如果`type="page"` 后续可用 分页标签 `pagination` 来组织分页显示 `{% pagination pages with show="5" %}`；搜索关键词 `q`，如果需要搜索内容，可以通过参数`q`来展示指定包含关键词的标题搜索内容如 `q="seo"` 呈现结果将只显示标题包含`seo`关键词的列表。

item 为 for循环体内的变量，可用的字段有：

- 文章ID `Id`
- 文章标题 `Title`
- 文章链接 `Link`
- 文章关键词 `Keywords`
- 文章描述 `Description`
- 文章内容 `Content`
- 文章分类ID `CategoryId`
- 文章浏览量 `Views`
- 文章封面图片 `Images`
- 文章封面首图 `Logo`
- 文章封面缩略图 `Thumb`
- 文章分类 `Category`
- 文章添加时间 `CreatedTime` 时间戳，需要使用格式化时间戳为日期格式 `{{stampToDate(item.CreatedTime, "2006-01-02")}}`
- 文章更新时间 `UpdatedTime` 时间戳，需要使用格式化时间戳为日期格式 `{{stampToDate(item.UpdatedTime, "2006-01-02")}}`

```twig
{# list 列表展示 #}
<div>
{% articleList articles with type="list" order="views desc" category="1" limit="10" %}
    {% for item in articles %}
    <li class="item layui-flex">
        <a href="{{item.Link}}" class="link flex-item">
            <h5 class="title">{{item.Title}}</h5>
            <div class="description">{{item.Description}}</div>
            <div class="meta">
                <span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
                <span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
                <span>{{item.Views}} 阅读</span>
            </div>
        </a>
        {% if item.Thumb %}
        <a href="{{item.Link}}" class="thumb">
            <img class="thumb-image" alt="{{item.Title}}" src="{{item.Thumb}}">
        </a>
        {% endif %}
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endarticleList %}
</div>

{# related 相关文章列表展示 #}
<div>
{% articleList articles with type="related" limit="10" %}
    {% for item in articles %}
    <li class="item layui-flex">
        <a href="{{item.Link}}" class="link flex-item">
            <h5 class="title">{{item.Title}}</h5>
            <div class="description">{{item.Description}}</div>
            <div class="meta">
                <span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
                <span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
                <span>{{item.Views}} 阅读</span>
            </div>
        </a>
        {% if item.Thumb %}
        <a href="{{item.Link}}" class="thumb">
            <img class="thumb-image" alt="{{item.Title}}" src="{{item.Thumb}}">
        </a>
        {% endif %}
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endarticleList %}
</div>

{# page 分页列表展示 #}
<div>
{% articleList articles with type="page" limit="10" %}
    {% for item in articles %}
    <li class="item layui-flex">
        <a href="{{item.Link}}" class="link flex-item">
            <h5 class="title">{{item.Title}}</h5>
            <div class="description">{{item.Description}}</div>
            <div class="meta">
                <span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
                <span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
                <span>{{item.Views}} 阅读</span>
            </div>
        </a>
        {% if item.Thumb %}
        <a href="{{item.Link}}" class="thumb">
            <img class="thumb-image" alt="{{item.Title}}" src="{{item.Thumb}}">
        </a>
        {% endif %}
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endarticleList %}
</div>

{# page 搜索指定关键词分页列表展示 #}
<div>
{% articleList articles with type="page" q="seo" limit="10" %}
    {% for item in articles %}
    <li class="item layui-flex">
        <a href="{{item.Link}}" class="link flex-item">
            <h5 class="title">{{item.Title}}</h5>
            <div class="description">{{item.Description}}</div>
            <div class="meta">
                <span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
                <span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
                <span>{{item.Views}} 阅读</span>
            </div>
        </a>
        {% if item.Thumb %}
        <a href="{{item.Link}}" class="thumb">
            <img class="thumb-image" alt="{{item.Title}}" src="{{item.Thumb}}">
        </a>
        {% endif %}
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endarticleList %}
</div>

<div class="layui-box layui-laypage layui-laypage-default">
    {% pagination pages with show="4" %}
        <a class="layui-laypage-first {% if pages.FirstPage.IsCurrent %}layui-laypage-curr{% endif %}" href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a>
        {% if pages.PrevPage %}
        <a class="layui-laypage-prev" href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a>
        {% endif %}
        {% for item in pages.Pages %}
        <a class="{% if item.IsCurrent %}layui-laypage-curr{% endif %}" href="{{item.Link}}">{{item.Name}}</a>
        {% endfor %}
        {% if pages.NextPage %}
        <a class="layui-laypage-next" href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a>
        {% endif %}
        <a class="layui-laypage-last {% if pages.LastPage.IsCurrent %}layui-laypage-curr{% endif %}" href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a>
    {% endpagination %}
</div>
```

## 文章详情标签
说明：用于获取文章详情数据

使用方法：`{% articleDetail with name="变量名称" id="1" %}` 变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。articleDetail 支持的参数有： `id`，id 不是必须的，默认会获取当前文章。如果需要指定文章，可以通过设置id来达到目的。

name 参数可用的字段有：

- 文章标题 `Title`
- 文章链接 `Link`
- 文章关键词 `Keywords`
- 文章描述 `Description`
- 文章内容 `Content`
- 文章分类ID `CategoryId`
- 文章浏览量 `Views`
- 文章封面图片 `Images`
- 文章封面首图 `Logo`
- 文章封面缩略图 `Thumb`
- 文章分类 `Category`
- 文章ID `Id`
- 文章添加时间 `CreatedTime`
- 文章更新时间 `UpdatedTime`

### 文章标题 `Title`
标签用法：`{% articleDetail with name="Title" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章标题：{% articleDetail with name="Title" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章标题：{% articleDetail with name="Title" id="1" %}</div>
{# 自定义字段名称 #}
<div>文章标题：{% articleDetail articleTitle with name="Title" %}{{articleTitle}}</div>
<div>文章标题：{% articleDetail articleTitle with name="Title" id="1" %}{{articleTitle}}</div>
```

### 文章链接 `Link`
标签用法：`{% articleDetail with name="Link" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章链接：{% articleDetail with name="Link" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章链接：{% articleDetail with name="Link" id="1" %}</div>
{# 自定义字段名称 #}
<div>文章链接：{% articleDetail articleLink with name="Link" %}{{articleLink}}</div>
<div>文章链接：{% articleDetail articleLink with name="Link" id="1" %}{{articleLink}}</div>
```

### 文章描述 `Description`
标签用法：`{% articleDetail with name="Description" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章描述：{% articleDetail with name="Description" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章描述：{% articleDetail with name="Description" id="1" %}</div>
{# 自定义字段名称 #}
<div>文章描述：{% articleDetail articleDescription with name="Description" %}{{articleDescription}}</div>
<div>文章描述：{% articleDetail articleDescription with name="Description" id="1" %}{{articleDescription}}</div>
```

### 文章内容 `Content`
标签用法：`{% articleDetail with name="Content" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章内容：{% articleDetail with name="Content" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章内容：{% articleDetail with name="Content" id="1" %}</div>
{# 自定义字段名称 #}
<div>文章内容：{% articleDetail articleContent with name="Content" %}{{articleContent|safe}}</div>
<div>文章内容：{% articleDetail articleContent with name="Content" id="1" %}{{articleContent|safe}}</div>
```

### 文章分类ID `CategoryId`
标签用法：`{% articleDetail with name="CategoryId" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章分类ID：{% articleDetail with name="CategoryId" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章分类ID：{% articleDetail with name="CategoryId" id="1" %}</div>
{# 自定义字段名称 #}
<div>文章分类ID：{% articleDetail articleCategoryId with name="CategoryId" %}{{articleCategoryId}}</div>
<div>文章分类ID：{% articleDetail articleCategoryId with name="CategoryId" id="1" %}{{articleCategoryId}}</div>
```

### 文章浏览量 `Views`
标签用法：`{% articleDetail with name="Views" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章浏览量：{% articleDetail with name="Views" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章浏览量：{% articleDetail with name="Views" id="1" %}</div>
{# 自定义字段名称 #}
<div>文章浏览量：{% articleDetail articleViews with name="Views" %}{{articleViews}}</div>
<div>文章浏览量：{% articleDetail articleViews with name="Views" id="1" %}{{articleViews}}</div>
```

### 文章封面首图 `Logo`
标签用法：`{% articleDetail with name="Logo" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章封面首图：<img src="{% articleDetail with name="Logo" %}" alt=""/></div>
{# 获取指定文章id的文章字段 #}
<div>文章封面首图：<img src="{% articleDetail with name="Logo" id="1" %}" alt=""/></div>
{# 自定义字段名称 #}
<div>文章封面首图：<img src="{% articleDetail articleLogo with name="Logo" %}{{articleLogo}}" alt=""/></div>
<div>文章封面首图：<img src="{% articleDetail articleLogo with name="Logo" id="1" %}{{articleLogo}}" alt=""/></div>
```

### 文章封面缩略图 `Thumb`
标签用法：`{% articleDetail with name="Thumb" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章封面缩略图：<img src="{% articleDetail with name="Thumb" %}" alt=""/></div>
{# 获取指定文章id的文章字段 #}
<div>文章封面缩略图：<img src="{% articleDetail with name="Thumb" id="1" %}" alt=""/></div>
{# 自定义字段名称 #}
<div>文章封面缩略图：<img src="{% articleDetail articleThumb with name="Thumb" %}{{articleThumb}}" alt=""/></div>
<div>文章封面缩略图：<img src="{% articleDetail articleThumb with name="Thumb" id="1" %}{{articleThumb}}" alt=""/></div>
```

### 文章封面图片 `Images`
Images 是一组图片，因此需要使用自定义方式来获取并循环输出

标签用法：`{% articleDetail articleImages with name="Images" %}`
```twig
{# 自定义字段名称 #}
<div>文章封面图片：
    {% articleDetail articleImages with name="Images" %}
    {% for item in articleImages %}
        <img src="{{item}}" alt=""/>
    {% endfor %}
</div>
<div>文章封面图片：
    {% articleDetail articleImages with name="Images" id="1" %}
    {% for item in articleImages %}
        <img src="{{item}}" alt=""/>
    {% endfor %}
</div>
```

### 文章ID `Id`
标签用法：`{% articleDetail with name="Id" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>分类标题：{% articleDetail with name="Id" %}</div>
{# 获取指定文章id的文章字段 #}
<div>分类标题：{% articleDetail with name="Id" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类标题：{% articleDetail articleId with name="Id" %}{{articleId}}</div>
<div>分类标题：{% articleDetail articleId with name="Id" id="1" %}{{articleId}}</div>
```

### 文章添加时间 `CreatedTime`
CreatedTime 支持预格式化时间。用`2006-01-02`表示年-月-日，用`15:04::05`表示时分秒。如需要显示格式为 2021年06月30日，可以写成`format="2006年01月02日"`，如需要显示格式为 2021/06/30 12:30，可以写成`format="2006/01/02 15:04"`。如果不设置format，在默认用法下，它会自动被格式化为`2006-01-02`。

标签用法：`{% articleDetail with name="CreatedTime" format="2006-01-02 15:04" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章添加时间：{% articleDetail with name="CreatedTime" %}</div>
<div>文章添加时间：{% articleDetail with name="CreatedTime" format="2006-01-02 15:04" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章添加时间：{% articleDetail with name="CreatedTime" id="1" %}</div>
<div>文章添加时间：{% articleDetail with name="CreatedTime" id="1" format="2006-01-02 15:04" %}</div>
{# 自定义字段名称 #}
<div>文章添加时间：{% articleDetail articleCreatedTime with name="CreatedTime" %}{{articleCreatedTime}}</div>
<div>文章添加时间：{% articleDetail articleCreatedTime with name="CreatedTime" id="1" %}{{articleCreatedTime}}</div>
<div>文章添加时间：{% articleDetail articleCreatedTime with name="CreatedTime" format="2006-01-02" %}{{articleCreatedTime}}</div>
<div>文章添加时间：{% articleDetail articleCreatedTime with name="CreatedTime" id="1" format="2006-01-02 15:04" %}{{articleCreatedTime}}</div>
```

### 文章更新时间 `UpdatedTime`
UpdatedTime 支持预格式化时间。用`2006-01-02`表示年-月-日，用`15:04::05`表示时分秒。如需要显示格式为 2021年06月30日，可以写成`format="2006年01月02日"`，如需要显示格式为 2021/06/30 12:30，可以写成`format="2006/01/02 15:04"`。如果不设置format，在默认用法下，它会自动被格式化为`2006-01-02`。

标签用法：`{% articleDetail with name="UpdatedTime" format="2006-01-02 15:04" %}`
```twig
{# 默认用法，自动获取当前页面文章 #}
<div>文章更新时间：{% articleDetail with name="UpdatedTime" %}</div>
<div>文章更新时间：{% articleDetail with name="UpdatedTime" format="2006-01-02 15:04" %}</div>
{# 获取指定文章id的文章字段 #}
<div>文章更新时间：{% articleDetail with name="UpdatedTime" id="1" %}</div>
<div>文章更新时间：{% articleDetail with name="UpdatedTime" id="1" format="2006-01-02 15:04" %}</div>
{# 自定义字段名称 #}
<div>文章更新时间：{% articleDetail articleUpdatedTime with name="UpdatedTime" %}{{articleUpdatedTime}}</div>
<div>文章更新时间：{% articleDetail articleUpdatedTime with name="UpdatedTime" id="1" %}{{articleUpdatedTime}}</div>
<div>文章更新时间：{% articleDetail articleUpdatedTime with name="UpdatedTime" format="2006-01-02" %}{{articleUpdatedTime}}</div>
<div>文章更新时间：{% articleDetail articleUpdatedTime with name="UpdatedTime" id="1" format="2006-01-02 15:04" %}{{articleUpdatedTime}}</div>
```

### 文章分类 `Category`
使用方法参看分类详情标签

### 文章参数
使用方法参看文章参数标签

## 产品列表标签
说明：用于获取产品常规列表、相关产品列表、产品分页列表

使用方法：`{% productList 变量名称 with categoryId="1" order="id desc|views desc" type="page|list" q="搜索关键词" %}` 如将变量定义为 products `{% productList products with type="page" %}...{% endproductList %}` productList 支持的参数有： `categoryId`，可以获取指定分类的产品列表如 `categoryId="1"` 获取产品分类ID为1的产品列表；`order` 可以指定产品显示的排序规则，支持依据 最新产品排序 `order="id desc"`、浏览量最多产品排序 `order="views desc"`；显示数量 `limit`数量的列表，比如`limit="10"`则只会显示10条； 列表类型 `type`，支持按 page、list、related 方式列出。默认值为list，`type="list"` 时，只会显示 limit 指定的数量，如果`type="page"` 后续可用 分页标签 `pagination` 来组织分页显示 `{% pagination pages with show="5" %}`；搜索关键词 `q`，如果需要搜索内容，可以通过参数`q`来展示指定包含关键词的标题搜索内容如 `q="seo"` 呈现结果将只显示标题包含`seo`关键词的列表。

item 为 for循环体内的变量，可用的字段有：

- 产品ID `Id`
- 产品标题 `Title`
- 产品链接 `Link`
- 产品关键词 `Keywords`
- 产品描述 `Description`
- 产品内容 `Content`
- 产品分类ID `CategoryId`
- 产品价格 `Price`
- 产品库存 `Stock`
- 产品浏览量 `Views`
- 产品封面图片 `Images`
- 产品封面首图 `Logo`
- 产品封面缩略图 `Thumb`
- 产品分类 `Category`
- 产品添加时间 `CreatedTime` 时间戳，需要使用格式化时间戳为日期格式 `{{stampToDate(item.CreatedTime, "2006-01-02")}}`
- 产品更新时间 `UpdatedTime` 时间戳，需要使用格式化时间戳为日期格式 `{{stampToDate(item.UpdatedTime, "2006-01-02")}}`

```twig
{# list 列表展示 #}
<div>
{% productList products with type="list" categoryId="1" order="views desc" %}
    {% for item in products %}
    <li class="layui-col-xs6 layui-col-sm4 layui-col-md3">
        <a href="{{item.Link}}" class="item">
            <div class="thumb">
                <img alt="{{item.Title}}" src="{{item.Thumb}}"/>
                <div class="tips">查看详情</div>
            </div>
            <div class="layout">
                {% if item.Price or item.Stock %}
                <div class="meta">
                    <span class="price"><i>￥</i>{{item.Price|floatformat:2}}元</span>
                    {% if item.Stock %}
                    <span class="stock">{{item.Stock}}件</span>
                    {% endif %}
                </div>
                {% endif %}
                <h5 class="title">{{item.Title}}</h5>
            </div>
        </a>
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endproductList %}
</div>

{# page 分页列表展示 #}
<div>
{% productList products with type="page" %}
    {% for item in products %}
    <li class="layui-col-xs6 layui-col-sm4 layui-col-md3">
        <a href="{{item.Link}}" class="item">
            <div class="thumb">
                <img alt="{{item.Title}}" src="{{item.Thumb}}"/>
                <div class="tips">查看详情</div>
            </div>
            <div class="layout">
                {% if item.Price or item.Stock %}
                <div class="meta">
                    <span class="price"><i>￥</i>{{item.Price|floatformat:2}}元</span>
                    {% if item.Stock %}
                    <span class="stock">{{item.Stock}}件</span>
                    {% endif %}
                </div>
                {% endif %}
                <h5 class="title">{{item.Title}}</h5>
            </div>
        </a>
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endproductList %}
</div>

{# page 搜索指定关键词分页列表展示 #}
<div>
{% productList products with type="page" q="seo" %}
    {% for item in products %}
    <li class="layui-col-xs6 layui-col-sm4 layui-col-md3">
        <a href="{{item.Link}}" class="item">
            <div class="thumb">
                <img alt="{{item.Title}}" src="{{item.Thumb}}"/>
                <div class="tips">查看详情</div>
            </div>
            <div class="layout">
                {% if item.Price or item.Stock %}
                <div class="meta">
                    <span class="price"><i>￥</i>{{item.Price|floatformat:2}}元</span>
                    {% if item.Stock %}
                    <span class="stock">{{item.Stock}}件</span>
                    {% endif %}
                </div>
                {% endif %}
                <h5 class="title">{{item.Title}}</h5>
            </div>
        </a>
    </li>
    {% empty %}
    <li class="item empty">
        该列表没有任何内容
    </li>
    {% endfor %}
{% endproductList %}
</div>

<div class="pagination">
    {% pagination pages with show="5" %}
    <ul>
        <li>总数：{{pages.TotalItems}}条，总共：{{pages.TotalPages}}页，当前第{{pages.CurrentPage}}页</li>
        <li class="page-item {% if pages.FirstPage.IsCurrent %}active{% endif %}"><a href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a></li>
        {% if pages.PrevPage %}
            <li class="page-item"><a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a></li>
        {% endif %}
        {% for item in pages.Pages %}
            <li class="page-item {% if item.IsCurrent %}active{% endif %}"><a href="{{item.Link}}">{{item.Name}}</a></li>
        {% endfor %}
        {% if pages.NextPage %}
            <li class="page-item"><a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a></li>
        {% endif %}
        <li class="page-item {% if pages.LastPage.IsCurrent %}active{% endif %}"><a href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a></li>
    </ul>
    {% endpagination %}
</div>
```

## 产品详情标签
说明：用于获取产品详情数据

使用方法：`{% productDetail with name="变量名称" id="1" %}` 变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。productDetail 支持的参数有： `id`，id 不是必须的，默认会获取当前产品。如果需要指定产品，可以通过设置id来达到目的。

name 参数可用的字段有：

- 产品标题 `Title`
- 产品链接 `Link`
- 产品关键词 `Keywords`
- 产品描述 `Description`
- 产品内容 `Content`
- 产品分类ID `CategoryId`
- 产品价格 `Price`
- 产品库存 `Stock`
- 产品浏览量 `Views`
- 产品封面图片 `Images`
- 产品封面首图 `Logo`
- 产品封面缩略图 `Thumb`
- 产品ID `Id`
- 产品添加时间 `CreatedTime`
- 产品更新时间 `UpdatedTime`

### 产品标题 `Title`
标签用法：`{% productDetail with name="Title" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品标题：{% productDetail with name="Title" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品标题：{% productDetail with name="Title" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品标题：{% productDetail productTitle with name="Title" %}{{productTitle}}</div>
<div>产品标题：{% productDetail productTitle with name="Title" id="1" %}{{productTitle}}</div>
```

### 产品链接 `Link`
标签用法：`{% productDetail with name="Link" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品链接：{% productDetail with name="Link" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品链接：{% productDetail with name="Link" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品链接：{% productDetail productLink with name="Link" %}{{productLink}}</div>
<div>产品链接：{% productDetail productLink with name="Link" id="1" %}{{productLink}}</div>
```

### 产品描述 `Description`
标签用法：`{% productDetail with name="Description" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品描述：{% productDetail with name="Description" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品描述：{% productDetail with name="Description" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品描述：{% productDetail productDescription with name="Description" %}{{productDescription}}</div>
<div>产品描述：{% productDetail productDescription with name="Description" id="1" %}{{productDescription}}</div>
```

### 产品内容 `Content`
标签用法：`{% productDetail with name="Content" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品内容：{% productDetail with name="Content" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品内容：{% productDetail with name="Content" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品内容：{% productDetail productContent with name="Content" %}{{productContent|safe}}</div>
<div>产品内容：{% productDetail productContent with name="Content" id="1" %}{{productContent|safe}}</div>
```

### 产品分类ID `CategoryId`
标签用法：`{% productDetail with name="CategoryId" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品分类ID：{% productDetail with name="CategoryId" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品分类ID：{% productDetail with name="CategoryId" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品分类ID：{% productDetail productCategoryId with name="CategoryId" %}{{productCategoryId}}</div>
<div>产品分类ID：{% productDetail productCategoryId with name="CategoryId" id="1" %}{{productCategoryId}}</div>
```

### 产品价格 `Price`
标签用法：`{% productDetail with name="Stock" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品分类ID：{% productDetail with name="CategoryId" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品分类ID：{% productDetail with name="CategoryId" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品分类ID：{% productDetail productCategoryId with name="CategoryId" %}{{productCategoryId}}</div>
<div>产品分类ID：{% productDetail productCategoryId with name="CategoryId" id="1" %}{{productCategoryId}}</div>
```

### 产品库存 `Stock`
标签用法：`{% productDetail with name="Stock" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品库存：{% productDetail with name="Stock" %}件</div>
{# 获取指定产品id的产品字段 #}
<div>产品库存：{% productDetail with name="Stock" id="1" %}件</div>
{# 自定义字段名称 #}
<div>产品库存：{% productDetail productStock with name="CategoryId" %}{{productStock}}件</div>
<div>产品库存：{% productDetail productStock with name="CategoryId" id="1" %}{{productStock}}件</div>
```

### 产品浏览量 `Views`
标签用法：`{% productDetail with name="Views" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品浏览量：{% productDetail with name="Views" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品浏览量：{% productDetail with name="Views" id="1" %}</div>
{# 自定义字段名称 #}
<div>产品浏览量：{% productDetail productViews with name="Views" %}{{productViews}}</div>
<div>产品浏览量：{% productDetail productViews with name="Views" id="1" %}{{productViews}}</div>
```

### 产品封面首图 `Logo`
标签用法：`{% productDetail with name="Logo" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品封面首图：<img src="{% productDetail with name="Logo" %}" alt=""/></div>
{# 获取指定产品id的产品字段 #}
<div>产品封面首图：<img src="{% productDetail with name="Logo" id="1" %}" alt=""/></div>
{# 自定义字段名称 #}
<div>产品封面首图：<img src="{% productDetail productLogo with name="Logo" %}{{productLogo}}" alt=""/></div>
<div>产品封面首图：<img src="{% productDetail productLogo with name="Logo" id="1" %}{{productLogo}}" alt=""/></div>
```

### 产品封面缩略图 `Thumb`
标签用法：`{% productDetail with name="Thumb" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品封面缩略图：<img src="{% productDetail with name="Thumb" %}" alt=""/></div>
{# 获取指定产品id的产品字段 #}
<div>产品封面缩略图：<img src="{% productDetail with name="Thumb" id="1" %}" alt=""/></div>
{# 自定义字段名称 #}
<div>产品封面缩略图：<img src="{% productDetail productThumb with name="Thumb" %}{{productThumb}}" alt=""/></div>
<div>产品封面缩略图：<img src="{% productDetail productThumb with name="Thumb" id="1" %}{{productThumb}}" alt=""/></div>
```

### 产品封面图片 `Images`
Images 是一组图片，因此需要使用自定义方式来获取并循环输出

标签用法：`{% productDetail productImages with name="Images" %}`
```twig
{# 自定义字段名称 #}
<div>产品封面图片：
    {% productDetail productImages with name="Images" %}
    {% for item in productImages %}
        <img src="{{item}}" alt=""/>
    {% endfor %}
</div>
<div>产品封面图片：
    {% productDetail productImages with name="Images" id="1" %}{{productImages}}
    {% for item in productImages %}
        <img src="{{item}}" alt=""/>
    {% endfor %}
</div>
```

### 产品ID `Id`
标签用法：`{% productDetail with name="Id" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>分类标题：{% productDetail with name="Id" %}</div>
{# 获取指定产品id的产品字段 #}
<div>分类标题：{% productDetail with name="Id" id="1" %}</div>
{# 自定义字段名称 #}
<div>分类标题：{% productDetail productId with name="Id" %}{{productId}}</div>
<div>分类标题：{% productDetail productId with name="Id" id="1" %}{{productId}}</div>
```

### 产品添加时间 `CreatedTime`
CreatedTime 支持预格式化时间。用`2006-01-02`表示年-月-日，用`15:04::05`表示时分秒。如需要显示格式为 2021年06月30日，可以写成`format="2006年01月02日"`，如需要显示格式为 2021/06/30 12:30，可以写成`format="2006/01/02 15:04"`。如果不设置format，在默认用法下，它会自动被格式化为`2006-01-02`。

标签用法：`{% productDetail with name="CreatedTime" format="2006-01-02 15:04" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品添加时间：{% productDetail with name="CreatedTime" %}</div>
<div>产品添加时间：{% productDetail with name="CreatedTime" format="2006-01-02 15:04" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品添加时间：{% productDetail with name="CreatedTime" id="1" %}</div>
<div>产品添加时间：{% productDetail with name="CreatedTime" id="1" format="2006-01-02 15:04" %}</div>
{# 自定义字段名称 #}
<div>产品添加时间：{% productDetail productCreatedTime with name="CreatedTime" %}{{productCreatedTime}}</div>
<div>产品添加时间：{% productDetail productCreatedTime with name="CreatedTime" id="1" %}{{productCreatedTime}}</div>
<div>产品添加时间：{% productDetail productCreatedTime with name="CreatedTime" format="2006-01-02" %}{{productCreatedTime}}</div>
<div>产品添加时间：{% productDetail productCreatedTime with name="CreatedTime" id="1" format="2006-01-02 15:04" %}{{productCreatedTime}}</div>
```

### 产品更新时间 `UpdatedTime`
UpdatedTime 支持预格式化时间。用`2006-01-02`表示年-月-日，用`15:04::05`表示时分秒。如需要显示格式为 2021年06月30日，可以写成`format="2006年01月02日"`，如需要显示格式为 2021/06/30 12:30，可以写成`format="2006/01/02 15:04"`。如果不设置format，在默认用法下，它会自动被格式化为`2006-01-02`。

标签用法：`{% productDetail with name="UpdatedTime" format="2006-01-02 15:04" %}`
```twig
{# 默认用法，自动获取当前页面产品 #}
<div>产品更新时间：{% productDetail with name="UpdatedTime" %}</div>
<div>产品更新时间：{% productDetail with name="UpdatedTime" format="2006-01-02 15:04" %}</div>
{# 获取指定产品id的产品字段 #}
<div>产品更新时间：{% productDetail with name="UpdatedTime" id="1" %}</div>
<div>产品更新时间：{% productDetail with name="UpdatedTime" id="1" format="2006-01-02 15:04" %}</div>
{# 自定义字段名称 #}
<div>产品更新时间：{% productDetail productUpdatedTime with name="UpdatedTime" %}{{productUpdatedTime}}</div>
<div>产品更新时间：{% productDetail productUpdatedTime with name="UpdatedTime" id="1" %}{{productUpdatedTime}}</div>
<div>产品更新时间：{% productDetail productUpdatedTime with name="UpdatedTime" format="2006-01-02" %}{{productUpdatedTime}}</div>
<div>产品更新时间：{% productDetail productUpdatedTime with name="UpdatedTime" id="1" format="2006-01-02 15:04" %}{{productUpdatedTime}}</div>
```

### 产品分类 `Category`
使用方法参看分类详情标签。

## 单页列表标签
说明：用于获取单页列表

使用方法：`{% pageList 变量名称 %}` 如将变量定义为 pages `{% pageList pages %}...{% endpageList %}` pageList 不支持参数，因此该标签会获取所有的页面。如果需要排除某些页面，可以在后续的for循环中，剔除不需要的页面。

item 为for循环体内的变量，可用的字段有：

- 单页标题 `Title`
- 单页链接 `Link`
- 单页描述 `Description`
- 单页内容 `Content`
- 单页ID `Id`

```twig
<ul>
{% pageList pages %}
    {% for item in pages %}
    <li>
        <a href="{{ item.Link }}">{{item.Title}}</a>
        <a href="{{ item.Link }}">
            <span>单页ID：{{item.Id}}</span>
            <span>单页名称：{{item.Title}}</span>
            <span>单页链接：{{item.Link}}</span>
            <span>单页描述：{{item.Description}}</span>
            <span>单页内容：{{item.Content|safe}}</span>
        </a>
    </li>
    {% endfor %}
{% endpageList %}
</ul>
{# 排除id为1的页面 #}
{% pageList pages %}
    {% for item in pages %}
    {% if item.Id != 1 %}
    <li>
        <a href="{{ item.Link }}">{{item.Title}}</a>
        <a href="{{ item.Link }}">
            <span>单页ID：{{item.Id}}</span>
            <span>单页名称：{{item.Title}}</span>
            <span>单页链接：{{item.Link}}</span>
            <span>单页描述：{{item.Description}}</span>
            <span>单页内容：{{item.Content|safe}}</span>
        </a>
    </li>
    {% endif %}
    {% endfor %}
</ul>
{% endpageList %}
```

## 单页详情标签
说明：用于获取单页详情数据

使用方法：`{% pageDetail with name="变量名称" id="1" %}` 变量名称不是必须的，设置了变量名称后，后续可以通过变量名称来调用，而不设置变量名称，则是直接输出结果。
pageDetail 支持的参数有： `id`，id 不是必须的，默认会获取当前单页。如果需要指定单页，可以通过设置id来达到目的。

name 参数可用的字段有：

- 单页标题 `Title`
- 单页链接 `Link`
- 单页描述 `Description`
- 单页内容 `Content`
- 单页ID `Id`

### 单页标题 `Title`
标签用法：`{% pageDetail with name="Title" %}`
```twig
{# 默认用法，自动获取当前页面单页 #}
<div>单页标题：{% pageDetail with name="Title" %}</div>
{# 获取指定单页id的单页字段 #}
<div>单页标题：{% pageDetail with name="Title" id="1" %}</div>
{# 自定义字段名称 #}
<div>单页标题：{% pageDetail pageTitle with name="Title" %}{{pageTitle}}</div>
<div>单页标题：{% pageDetail pageTitle with name="Title" id="1" %}{{pageTitle}}</div>
```

### 单页链接 `Link`
标签用法：`{% pageDetail with name="Link" %}`
```twig
{# 默认用法，自动获取当前页面单页 #}
<div>单页链接：{% pageDetail with name="Link" %}</div>
{# 获取指定单页id的单页字段 #}
<div>单页链接：{% pageDetail with name="Link" id="1" %}</div>
{# 自定义字段名称 #}
<div>单页链接：{% pageDetail pageLink with name="Link" %}{{pageLink}}</div>
<div>单页链接：{% pageDetail pageLink with name="Link" id="1" %}{{pageLink}}</div>
```

### 单页描述 `Description`
标签用法：`{% pageDetail with name="Description" %}`
```twig
{# 默认用法，自动获取当前页面单页 #}
<div>单页描述：{% pageDetail with name="Description" %}</div>
{# 获取指定单页id的单页字段 #}
<div>单页描述：{% pageDetail with name="Description" id="1" %}</div>
{# 自定义字段名称 #}
<div>单页描述：{% pageDetail pageDescription with name="Description" %}{{pageDescription}}</div>
<div>单页描述：{% pageDetail pageDescription with name="Description" id="1" %}{{pageDescription}}</div>
```

### 单页内容 `Content`
标签用法：`{% pageDetail with name="Content" %}`
```twig
{# 默认用法，自动获取当前页面单页 #}
<div>单页内容：{% pageDetail with name="Content" %}</div>
{# 获取指定单页id的单页字段 #}
<div>单页内容：{% pageDetail with name="Content" id="1" %}</div>
{# 自定义字段名称 #}
<div>单页内容：{% pageDetail pageContent with name="Content" %}{{pageContent|safe}}</div>
<div>单页内容：{% pageDetail pageContent with name="Content" id="1" %}{{pageContent|safe}}</div>
```

### 单页ID `Id`
标签用法：`{% pageDetail with name="Id" %}`
```twig
{# 默认用法，自动获取当前页面单页 #}
<div>单页ID：{% pageDetail with name="Id" %}</div>
{# 获取指定单页id的单页字段 #}
<div>单页ID：{% pageDetail with name="Id" id="1" %}</div>
{# 自定义字段名称 #}
<div>单页ID：{% pageDetail pageId with name="Id" %}{{pageId}}</div>
<div>单页ID：{% pageDetail pageId with name="Id" id="1" %}{{pageId}}</div>
```

## 评论标列表签
说明：用于获取文章、产品的评论列表、评论分页列表

使用方法：`{% commentList 变量名称 with itemType="article|product" itemId="1" type="page|list" %}` 如将变量定义为 comments `{% commentList comments with itemType="article" itemId="1" type="page" %}...{% endcommentList %}` commentList 支持的参数有： 评论类型 `itemType`，支持的值有：article、product，分别表示文章评论、产品评论；评论内容ID `itemId`，itemId 为指定的文章、产品ID；`order` 可以指定产品显示的排序规则，支持依据 id正序排序 `order="id desc"`、按id倒叙排序 `order="id desc"`；显示数量 `limit`数量的列表，比如`limit="10"`则只会显示10条；列表类型 `type`，支持按 page、list 方式列出。默认值为list，如果`type="page"` 后续可用 分页标签 `pagination` 来组织分页显示 `{% pagination pages with show="5" %}`。

item 为 for循环体内的变量，可用的字段有：

- 评论ID `Id`
- 评论类型 `ItemType` 可能的值有：article、product
- 类型内容ID `ItemId`
- 用户名 `UserName`
- 用户ID `UserId`
- 用户IP `Ip`
- 点赞数量 `VoteCount`
- 评论内容 `Content`
- 上级评论ID `ParentId`
- 审核状态 `Status` Status = 1 表示审核通过， status = 0 时审核中，不要显示出来
- 上级评论的对象数据 `Parent` Parent 包含上级评论的完整 item，字段和评论item相同
- 添加时间 `CreatedTime` 时间戳，需要使用格式化时间戳为日期格式 `{{stampToDate(item.CreatedTime, "2006-01-02")}}`

```twig
{# list 列表展示 #}
<div>
{% commentList comments with itemType="article" itemId=article.Id type="list" limit="6" %}
    {% for item in comments %}
    <div class="comment-item">
      <div class="item-user">
        <span class="user-name">
          {% if item.Status != 1 %}
          审核中：{{item.UserName|truncatechars:6}}
          {% else %}
          {{item.UserName}}
          {% endif %}
        </span>
        {% if item.Parent %}
        <span class="text">回复</span>
        <span class="user-name">
          {% if item.Status != 1 %}
          审核中：{{item.Parent.UserName|truncatechars:6}}
          {% else %}
          {{item.Parent.UserName}}
          {% endif %}
        </span>
        {% endif %}
        <span class="publish-time">{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
      </div>
      <div class="comment-content">
        {% if item.Parent %}
        <blockquote class="layui-elem-quote layui-quote-nm">
          {% if item.Parent.Status != 1 %}
          该内容正在审核中：{{item.Parent.Content|truncatechars:9}}
          {% else %}
          {{item.Parent.Content|truncatechars:100}}
          {% endif %}
        </blockquote>
        {% endif %}
        {% if item.Status != 1 %}
          该内容正在审核中：{{item.Content|truncatechars:9}}
        {% else %}
        {{item.Content}}
        {% endif %}
      </div>
      <div class="comment-control" data-id="{{item.Id}}" data-user="{{item.UserName}}">
        <a class="item" data-id="praise"><i class="layui-icon layui-icon-praise"></i>赞(<span class="vote-count">{{item.VoteCount}}</span>)</a>
        <a class="item" data-id=reply><i class="layui-icon layui-icon-release"></i>回复</a>
      </div>
    </div>
    {% endfor %}
{% endcommentList %}
</div>

{# page 分页列表展示 #}
<div>
{% commentList comments with itemType="article" itemId=article.Id type="page" limit="10" %}
    {% for item in comments %}
    <div class="comment-item">
      <div class="item-user">
        <span class="user-name">
          {% if item.Status != 1 %}
          审核中：{{item.UserName|truncatechars:6}}
          {% else %}
          {{item.UserName}}
          {% endif %}
        </span>
        {% if item.Parent %}
        <span class="text">回复</span>
        <span class="user-name">
          {% if item.Status != 1 %}
          审核中：{{item.Parent.UserName|truncatechars:6}}
          {% else %}
          {{item.Parent.UserName}}
          {% endif %}
        </span>
        {% endif %}
        <span class="publish-time">{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
      </div>
      <div class="comment-content">
        {% if item.Parent %}
        <blockquote class="layui-elem-quote layui-quote-nm">
          {% if item.Parent.Status != 1 %}
          该内容正在审核中：{{item.Parent.Content|truncatechars:9}}
          {% else %}
          {{item.Parent.Content|truncatechars:100}}
          {% endif %}
        </blockquote>
        {% endif %}
        {% if item.Status != 1 %}
          该内容正在审核中：{{item.Content|truncatechars:9}}
        {% else %}
        {{item.Content}}
        {% endif %}
      </div>
      <div class="comment-control" data-id="{{item.Id}}" data-user="{{item.UserName}}">
        <a class="item" data-id="praise"><i class="layui-icon layui-icon-praise"></i>赞(<span class="vote-count">{{item.VoteCount}}</span>)</a>
        <a class="item" data-id=reply><i class="layui-icon layui-icon-release"></i>回复</a>
      </div>
    </div>
    {% endfor %}
{% endcommentList %}
</div>

<div class="pagination">
    {% pagination pages with show="5" %}
    <ul>
        <li>总数：{{pages.TotalItems}}条，总共：{{pages.TotalPages}}页，当前第{{pages.CurrentPage}}页</li>
        <li class="page-item {% if pages.FirstPage.IsCurrent %}active{% endif %}"><a href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a></li>
        {% if pages.PrevPage %}
            <li class="page-item"><a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a></li>
        {% endif %}
        {% for item in pages.Pages %}
            <li class="page-item {% if item.IsCurrent %}active{% endif %}"><a href="{{item.Link}}">{{item.Name}}</a></li>
        {% endfor %}
        {% if pages.NextPage %}
            <li class="page-item"><a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a></li>
        {% endif %}
        <li class="page-item {% if pages.LastPage.IsCurrent %}active{% endif %}"><a href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a></li>
    </ul>
    {% endpagination %}
</div>
```

## 留言表单标签
说明：用于获取后台设置的留言表单

使用方法：`{% guestbook 变量名称 %}` 如将变量定义为fields `{% guestbook fields %}...{% endguestbook %}` 该标签不支持参数。

item 为 for循环体内的变量，可用的字段有:

- 表单名称 `Name`
- 表单变量 `FieldName`
- 表单类型 `Type` 表单类型有6种可能的值：文本类型 `text`、数字类型 `number`、多行文本类型 `textarea`、单项选择类型 `radio`、多项选择类型 `checkbox`、下拉选择类型 `select`。
- 是否必填 `Required` Required 值为 true 时，表示必填，Required 值为 false 时，表示可以不填。
- 表单默认值 `Content`
- 分割成数组的默认值 `Items` 当 表单类型为 单项选择类型 `radio`、多项选择类型 `checkbox`、下拉选择类型 `select` 时，它们的每一个选择项构成了一个 Items 数组，可以通过 for循环输出。

```twig
<form class="layui-form" onsubmit="return false;">
{% guestbook fields %}
    {% for item in fields %}
    <div class="layui-form-item">
        <label class="layui-form-label">{{item.Name}}</label>
        <div class="layui-input-block">
            {% if item.Type == "text" || item.Type == "number" %}
            <input type="{{item.Type}}" name="{{item.FieldName}}" {% if item.Required %}required lay-verify="required"{% endif %} placeholder="{{item.Content}}" autocomplete="off" class="layui-input">
            {% elif item.Type == "textarea" %}
            <textarea class="layui-textarea" name="{{item.FieldName}}" {% if item.Required %}required lay-verify="required"{% endif %} placeholder="{{item.Content}}" rows="5"></textarea>
            {% elif item.Type == "radio" %}
            {%- for val in item.Items %}
            <input type="{{item.Type}}" name="{{item.FieldName}}" value="{{val}}" title="{{val}}">
            {%- endfor %}
            {% elif item.Type == "checkbox" %}
            {%- for val in item.Items %}
            <input type="{{item.Type}}" name="{{item.FieldName}}[]" value="{{val}}" title="{{val}}">
            {%- endfor %}
            {% elif item.Type == "select" %}
            <select name="{{item.FieldName}}">
                {%- for val in item.Items %}
                <option value="{{val}}">{{val}}</option>
                {%- endfor %}
            </select>
            {% endif %}
        </div>
    </div>
    {% endfor %}
    <div class="layui-form-item">
        <div class="layui-input-block">
            <button class="layui-btn" lay-submit lay-filter="guestbook-submit">提交留言</button>
            <button type="reset" class="layui-btn layui-btn-primary">重置</button>
        </div>
    </div>
{% endguestbook %}
</form>
```

## 面包屑标签
说明：用于获取面包屑导航列表

使用方法：`{% breadcrumb 变量名称 with index="首页" title=true %}` 如将变量定义为 crumbs `{% breadcrumb crumbs with index="首页" title=true %}...{% endbreadcrumb %}` breadcrumb 支持2个参数，`title` 和 `index`，title 参数可以设置文章、产品详情面包屑是否显示文章、产品标题。如果设置了 `title=false` 则文章、产品标题将由 正文 二字替代，`title=true` 时，则显示完整的文章、产品标题，默认值为`title=true`；index 参数可以设置首页显示名称，默认值为 首页，如需改成其他可以设置，如 index="我的博客"。

item 为for循环体内的变量，可用的字段有：

- 链接名称 `Name`
- 链接地址 `Link`

```twig
{# 默认用法 #}
<div class="breadcrumb">
    {% breadcrumb crumbs %}
    <ul>
        {% for item in crumbs %}
            <li><a href="{{item.Link}}">{{item.Name}}</a></li>        
        {% endfor %}
    </ul>
    {% endbreadcrumb %}
</div>

{# 自定义index，不显示标题 #}
<div class="breadcrumb">
    {% breadcrumb crumbs with index="我的博客" title=false %}
    <ul>
        {% for item in crumbs %}
            <li><a href="{{item.Link}}">{{item.Name}}</a></li>        
        {% endfor %}
    </ul>
    {% endbreadcrumb %}
</div>
```

## 上一篇文章标签
说明：用于获取上一篇文章数据

使用方法：`{% prevArticle 变量名称 %}` 如将变量定义为 prev `{% prevArticle prev %}...{% endprevArticle %}` prevArticle 不支持参数。prevArticle 支持的字段有：

- 文章标题 `Title`
- 文章链接 `Link`
- 文章关键词 `Keywords`
- 文章描述 `Description`
- 文章分类ID `CategoryId`
- 文章浏览量 `Views`
- 文章封面首图 `Logo`
- 文章封面缩略图 `Thumb`
- 文章ID `Id`
- 文章添加时间 `CreatedTime`
- 文章更新时间 `UpdatedTime`

```twig
{% prevArticle prev %}
上一篇：
{% if prev %}
  <a href="{{prev.Link}}">{{prev.Title}}</a>
{% else %}
  没有了
{% endif %}
{% endprevArticle %}
```

## 下一篇文章标签
说明：用于获取下一篇文章数据

使用方法：`{% nextArticle 变量名称 %}` 如将变量定义为 next `{% nextArticle next %}...{% endnextArticle %}` nextArticle 不支持参数。nextArticle 支持的字段有：

- 文章标题 `Title`
- 文章链接 `Link`
- 文章关键词 `Keywords`
- 文章描述 `Description`
- 文章分类ID `CategoryId`
- 文章浏览量 `Views`
- 文章封面首图 `Logo`
- 文章封面缩略图 `Thumb`
- 文章ID `Id`
- 文章添加时间 `CreatedTime`
- 文章更新时间 `UpdatedTime`

```twig
{% nextArticle next %}
下一篇：
{% if next %}
  <a href="{{next.Link}}">{{next.Title}}</a>
{% else %}
  没有了
{% endif %}
{% endnextArticle %}
```

## 上一产品标签
说明：用于获取上一个产品数据

使用方法：`{% prevProduct 变量名称 %}` 如将变量定义为 prev `{% prevProduct prev %}...{% endprevProduct %}` prevProduct 不支持参数。prevProduct 支持的字段有：

- 产品标题 `Title`
- 产品链接 `Link`
- 产品关键词 `Keywords`
- 产品描述 `Description`
- 产品分类ID `CategoryId`
- 产品浏览量 `Views`
- 产品封面首图 `Logo`
- 产品封面缩略图 `Thumb`
- 产品ID `Id`
- 产品添加时间 `CreatedTime`
- 产品更新时间 `UpdatedTime`

```twig
{% prevProduct prev %}
上一篇：
{% if prev %}
  <a href="{{prev.Link}}">{{prev.Title}}</a>
{% else %}
  没有了
{% endif %}
{% endprevProduct %}
```

## 下一产品标签
说明：用于获取下一个产品数据

使用方法：`{% nextProduct 变量名称 %}` 如将变量定义为 next `{% nextProduct next %}...{% endnextProduct %}` nextProduct 不支持参数。nextProduct 支持的字段有：

- 产品标题 `Title`
- 产品链接 `Link`
- 产品关键词 `Keywords`
- 产品描述 `Description`
- 产品分类ID `CategoryId`
- 产品浏览量 `Views`
- 产品封面首图 `Logo`
- 产品封面缩略图 `Thumb`
- 产品ID `Id`
- 产品添加时间 `CreatedTime`
- 产品更新时间 `UpdatedTime`

```twig
{% nextProduct next %}
下一篇：
{% if next %}
  <a href="{{next.Link}}">{{next.Title}}</a>
{% else %}
  没有了
{% endif %}
{% endnextProduct %}
```

## 相关文文章标签
使用方法参看文章列表标签：`{% articleList articleRelations with type="related" %}`

## 相关产品标签
使用方法参看产品列表标签：`{% productList productRelations with type="related" %}`

## 分页标签
说明：用于获取文章列表、产品列表的分页信息

使用方法：`{% pagination 变量名称 with show="5" %}` 如将变量定义为 pages `{% pagination pages with show="5" %}...{% endpagination %}` pagination 支持 一个参数 `show`，可以设置如果指定数量页码的时候，最多显示多少页码。如 `show="5"` 可以最多显示5页。

pagination 可用的字段有：

- 总条数 `TotalItems`
- 总页码数 `TotalPages`
- 当前页码 `CurrentPage`
- 首页对象 `FirstPage`
- 末页对象 `LastPage`
- 上一页对象 `PrevPage`
- 下一页对象 `NextPage`
- 中间页码数组 `Pages`

其中 对象和数组对象 pageItem 可用的字段有：

- 页码名称 `Name`
- 页码链接 `Link`
- 是否当前页 `IsCurrent`

```twig
<div class="pagination">
    {% pagination pages with show="5" %}
    <ul>
        <li>总数：{{pages.TotalItems}}条，总共：{{pages.TotalPages}}页，当前第{{pages.CurrentPage}}页</li>
        <li class="page-item {% if pages.FirstPage.IsCurrent %}active{% endif %}"><a href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a></li>
        {% if pages.PrevPage %}
            <li class="page-item"><a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a></li>
        {% endif %}
        {% for item in pages.Pages %}
            <li class="page-item {% if item.IsCurrent %}active{% endif %}"><a href="{{item.Link}}">{{item.Name}}</a></li>
        {% endfor %}
        {% if pages.NextPage %}
            <li class="page-item"><a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a></li>
        {% endif %}
        <li class="page-item {% if pages.LastPage.IsCurrent %}active{% endif %}"><a href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a></li>
    </ul>
    {% endpagination %}
</div>
```

## 文章参数标签
说明：用于获取指定文章的后台设置的参数

使用方法：``{% articleParams 变量名称 with id="1" sorted=true %}`` 如将变量定义为 params `{% articleParams params with id="1" sorted=true %}...{% endarticleParams %}` articleParams 支持 `id`、`sorted` 参数。`id` 参数根据文章id获取指定的文章参数，默认获取当前文章页面的文章id。`sorted=false` 时，获取的是一个无序的map对象，`sorted=true` 时，获取是一个固定排序的数组。默认是固定排序的数组。因此需要使用自定义名称来获取并输出。

具体的可用字段根据后台设置的文章附加字段来决定。

```twig
{# 固定排序的数组 #}
<div>
    {% articleParams params %}
    <table class="layui-table">
        <colgroup>
            <col width="100">
            <col>
        </colgroup>
        <tbody>
        {% for item in params %}
        <tr>
            <td>{{item.Name}}</td>
            <td>
                {{item.Value}}
            </td>
        </tr>
        {% endfor %}
        </tbody>
    </table>
    {% endarticleParams %}
</div>
指定文章ID
{# 固定排序的数组 #}
<div>
    {% articleParams params with id="1" %}
    <table class="layui-table">
        <colgroup>
            <col width="100">
            <col>
        </colgroup>
        <tbody>
        {% for item in params %}
        <tr>
            <td>{{item.Name}}</td>
            <td>
                {{item.Value}}
            </td>
        </tr>
        {% endfor %}
        </tbody>
    </table>
    {% endarticleParams %}
</div>
{# 无序的map对象 #}
<div>
    {% articleParams params with sorted=false %}
        <div>{{params.yuedu.Name}}:{{params.yuedu.Value}}</div>
        <div>{{params.danxuan.Name}}:{{params.danxuan.Value}}</div>
        <div>{{params.duoxuan.Name}}:{{params.duoxuan.Value}}</div>
    {% endarticleParams %}
</div>
```

## 产品参数标签
说明：用于获取指定产品的后台设置的产品参数

使用方法：``{% productParams 变量名称 with id="1" sorted=true %}`` 如将变量定义为 params `{% productParams params with id="1" sorted=true %}...{% endproductParams %}` productParams 支持 `id`、`sorted` 参数。`id` 参数根据产品id获取指定的产品参数，默认获取当前产品页面的产品id。`sorted=false` 时，获取的是一个无序的map对象，`sorted=true` 时，获取是一个固定排序的数组。默认是固定排序的数组。因此需要使用自定义名称来获取并输出。

具体的可用字段根据后台设置的产品附加字段来决定。

```twig
{# 固定排序的数组 #}
<div>
    {% productParams params %}
    <table class="layui-table">
        <colgroup>
            <col width="100">
            <col>
        </colgroup>
        <tbody>
        {% for item in params %}
        <tr>
            <td>{{item.Name}}</td>
            <td>
                {{item.Value}}
            </td>
        </tr>
        {% endfor %}
        </tbody>
    </table>
    {% endproductParams %}
</div>
指定产品ID
{# 固定排序的数组 #}
<div>
    {% productParams params with id="1" %}
    <table class="layui-table">
        <colgroup>
            <col width="100">
            <col>
        </colgroup>
        <tbody>
        {% for item in params %}
        <tr>
            <td>{{item.Name}}</td>
            <td>
                {{item.Value}}
            </td>
        </tr>
        {% endfor %}
        </tbody>
    </table>
    {% endproductParams %}
</div>
{# 无序的map对象 #}
<div>
    {% productParams params with sorted=false %}
    <div>{{params.yuedu.Name}}:{{params.yuedu.Value}}</div>
    <div>{{params.danxuan.Name}}:{{params.danxuan.Value}}</div>
    <div>{{params.duoxuan.Name}}:{{params.duoxuan.Value}}</div>
    {% endproductParams %}
</div>
```

## 友情链接标签
说明：用于获取友情链接列表

使用方法：`{% linkList 变量名称 %}` 如将变量定义为 friendLinks `{% linkList friendLinks %}...{% endlinkList %}` linkList 不支持参数，将会获取所有的友情链接。

item 为for循环体内的变量，可用的字段有：：

- 链接名称 `Title`
- 链接地址 `Link`
- 链接地址 `Remark`
- 链接地址 `Nofollow`

```twig
{% linkList friendLinks %}
{% if friendLinks %}
<div class="friend-links">
    <span class="title">友情链接：</span>
    {% for item in friendLinks %}
    <a class="item" href="{{item.Link}}" {% if item.Nofollow == 1 %} rel="nofollow"{% endif %} target="_blank">{{item.Title}}</a>
    {% endfor %}
</div>
{% endif %}
{% endlinkList %}
```