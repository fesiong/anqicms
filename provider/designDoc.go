package provider

import "kandaoni.com/anqicms/response"

var DesignDocs = []response.DesignDocGroup{
	{
		Title: "模板制作帮助",
		Docs: []response.DesignDoc{
			{
				Title: "一些基本约定",
				Link:  "https://www.anqicms.com/help-design/116.html",
			},
			{
				Title: "目录和模板",
				Link:  "https://www.anqicms.com/help-design/117.html",
			},
			{
				Title: "标签和使用方法",
				Link:  "https://www.anqicms.com/help-design/118.html",
			},
		},
	},
	{
		Title: "常用标签",
		Docs: []response.DesignDoc{
			{
				Title: "系统设置标签",
				Link:  "https://www.anqicms.com/manual-normal/73.html",
			},
			{
				Title: "联系方式标签",
				Link:  "https://www.anqicms.com/manual-normal/74.html",
			},
			{
				Title: "万能TDK标签",
				Link:  "https://www.anqicms.com/manual-normal/75.html",
			},
			{
				Title: "导航列表标签",
				Link:  "https://www.anqicms.com/manual-normal/76.html",
			},
			{
				Title: "面包屑导航标签",
				Link:  "https://www.anqicms.com/manual-normal/87.html",
			},
			{
				Title: "统计代码标签",
				Link:  "https://www.anqicms.com/manual-normal/91.html",
			},
			{
				Title: "首页Banner标签",
				Link:  "https://www.anqicms.com/manual-normal/3317.html",
			},
		},
	},
	{
		Title: "分类页面标签",
		Docs: []response.DesignDoc{
			{
				Title: "分类列表标签",
				Link:  "https://www.anqicms.com/manual-category/77.html",
			},
			{
				Title: "分类详情标签",
				Link:  "https://www.anqicms.com/manual-category/78.html",
			},
			{
				Title: "单页列表标签",
				Link:  "https://www.anqicms.com/manual-category/83.html",
			},
			{
				Title: "单页详情标签",
				Link:  "https://www.anqicms.com/manual-category/84.html",
			},
			{
				Title: "文档模型详情标签",
				Link:  "https://www.anqicms.com/manual-normal/3489.html",
			},
		},
	},
	{
		Title: "文档标签",
		Docs: []response.DesignDoc{
			{
				Title: "文档列表标签",
				Link:  "https://www.anqicms.com/manual-archive/79.html",
			},
			{
				Title: "文档详情标签",
				Link:  "https://www.anqicms.com/manual-archive/80.html",
			},
			{
				Title: "上一篇文档标签",
				Link:  "https://www.anqicms.com/manual-archive/88.html",
			},
			{
				Title: "下一篇文档标签",
				Link:  "https://www.anqicms.com/manual-archive/89.html",
			},
			{
				Title: "相关文档标签",
				Link:  "https://www.anqicms.com/manual-archive/92.html",
			},
			{
				Title: "文档参数标签",
				Link:  "https://www.anqicms.com/manual-archive/95.html",
			},
			{
				Title: "文档参数筛选标签",
				Link:  "https://www.anqicms.com/manual-archive/96.html",
			},
		},
	},
	{
		Title: "文档Tag标签",
		Docs: []response.DesignDoc{
			{
				Title: "文档Tag列表标签",
				Link:  "https://www.anqicms.com/manual-tag/81.html",
			},
			{
				Title: "Tag文档列表标签",
				Link:  "https://www.anqicms.com/manual-tag/82.html",
			},
			{
				Title: "Tag详情标签",
				Link:  "https://www.anqicms.com/manual-tag/90.html",
			},
		},
	},
	{
		Title: "其他标签",
		Docs: []response.DesignDoc{
			{
				Title: "评论标列表签",
				Link:  "https://www.anqicms.com/manual-other/85.html",
			},
			{
				Title: "留言表单标签",
				Link:  "https://www.anqicms.com/manual-other/86.html",
			},
			{
				Title: "分页标签",
				Link:  "https://www.anqicms.com/manual-other/94.html",
			},
			{
				Title: "友情链接标签",
				Link:  "https://www.anqicms.com/manual-other/97.html",
			},
			{
				Title: "留言验证码使用标签",
				Link:  "https://www.anqicms.com/manual-other/139.html",
			},
		},
	},
	{
		Title: "通用模板标签",
		Docs: []response.DesignDoc{
			{
				Title: "其他辅助标签",
				Link:  "https://www.anqicms.com/manual-common/93.html",
			},
			{
				Title: "更多过滤器",
				Link:  "https://www.anqicms.com/manual-common/98.html",
			},
			{
				Title: "定义变量赋值标签",
				Link:  "https://www.anqicms.com/manual-common/99.html",
			},
			{
				Title: "格式化时间戳标签",
				Link:  "https://www.anqicms.com/manual-common/100.html",
			},
			{
				Title: "for循环遍历标签",
				Link:  "https://www.anqicms.com/manual-common/101.html",
			},
			{
				Title: "移除逻辑标签占用行",
				Link:  "https://www.anqicms.com/manual-common/102.html",
			},
			{
				Title: "算术运算标签",
				Link:  "https://www.anqicms.com/manual-common/103.html",
			},
			{
				Title: "if逻辑判断标签",
				Link:  "https://www.anqicms.com/manual-common/104.html",
			},
		},
	},
}

var DesignTplHelpers = []response.DesignDocGroup{
	{
		Title: "常用标签",
		Type:  "tag",
		Docs: []response.DesignDoc{
			{
				Title: "系统设置标签",
				Link:  "https://www.anqicms.com/manual-normal/73.html",
				Docs: []response.DesignDoc{
					{
						Title: "网站名称",
						Code:  `{% system with name="SiteName" %}`,
					},
					{
						Title: "网站Logo",
						Code:  `{% system with name="SiteLogo" %}`,
					},
					{
						Title: "网站备案号",
						Code:  `{% system with name="SiteIcp" %}`,
					},
					{
						Title: "版权内容",
						Code:  `{% system with name="SiteCopyright" %}`,
					},
					{
						Title: "网站首页地址",
						Code:  `{% system with name="BaseUrl" %}`,
					},
					{
						Title: "网站手机端地址",
						Code:  `{% system with name="MobileUrl" %}`,
					},
					{
						Title: "模板静态文件地址",
						Code:  `{% system with name="TemplateUrl" %}`,
					},
					{
						Title: "自定义参数",
						Code:  `{% system with name="自定义参数名称" %}`,
					},
				},
			},
			{
				Title: "联系方式标签",
				Link:  "https://www.anqicms.com/manual-normal/74.html",
				Docs: []response.DesignDoc{
					{
						Title: "联系人",
						Code:  `{% contact with name="UserName" %}`,
					},
					{
						Title: "联系电话",
						Code:  `{% contact with name="Cellphone" %}`,
					},
					{
						Title: "联系地址",
						Code:  `{% contact with name="Address" %}`,
					},
					{
						Title: "联系邮箱",
						Code:  `{% contact with name="Email" %}`,
					},
					{
						Title: "联系微信",
						Code:  `{% contact with name="Wechat" %}`,
					},
					{
						Title: "微信二维码",
						Code:  `<img src="{% contact with name="Qrcode" %}" />`,
					},
					{
						Title: "联系QQ",
						Code:  `{% contact with name="QQ" %}`,
					},
					{
						Title: "联系Facebook",
						Code:  `{% contact with name="Facebook" %}`,
					},
					{
						Title: "联系Twitter",
						Code:  `{% contact with name="Twitter" %}`,
					},
					{
						Title: "联系Tiktok",
						Code:  `{% contact with name="Tiktok" %}`,
					},
					{
						Title: "联系Pinterest",
						Code:  `{% contact with name="Pinterest" %}`,
					},
					{
						Title: "联系Linkedin",
						Code:  `{% contact with name="Linkedin" %}`,
					},
					{
						Title: "联系Instagram",
						Code:  `{% contact with name="Instagram" %}`,
					},
					{
						Title: "联系Youtube",
						Code:  `{% contact with name="Youtube" %}`,
					},
					{
						Title: "自定义参数",
						Code:  `{% contact with name="自定义参数名称" %}`,
					},
				},
			},
			{
				Title: "万能TDK标签",
				Link:  "https://www.anqicms.com/manual-normal/75.html",
				Docs: []response.DesignDoc{
					{
						Title: "页面标题",
						Code:  `{% tdk with name="Title" %}`,
					},
					{
						Title: "页面关键词",
						Code:  `{% tdk with name="Keywords" %}`,
					},
					{
						Title: "页面描述",
						Code:  `{% tdk with name="Description" %}`,
					},
					{
						Title: "页面的规范链接",
						Code:  `{% tdk with name="CanonicalUrl" %}`,
					},
				},
			},
			{
				Title: "导航列表标签",
				Link:  "https://www.anqicms.com/manual-normal/76.html",
				Docs: []response.DesignDoc{
					{
						Title: "一级导航",
						Code: `{% navList navs %}
						<ul>
							{%- for item in navs %}
								<li class="{% if item.IsCurrent %}active{% endif %}">
									<a href="{{ item.Link }}">{{item.Title}}</a>
								</li>
							{% endfor %}
						</ul>
						{% endnavList %}`,
					},
					{
						Title: "一二级导航",
						Code: `{% navList navs %}
						<ul>
							{%- for item in navs %}
								<li class="{% if item.IsCurrent %}active{% endif %}">
									<a href="{{ item.Link }}">{{item.Title}}</a>
									{%- if item.NavList %}
									<dl>
										{%- for inner in item.NavList %}
											<dd class="{% if inner.IsCurrent %}active{% endif %}">
												<a href="{{ inner.Link }}">{{inner.Title}}</a>
											</dd>
										{% endfor %}
									</dl>
									{% endif %}
								</li>
							{% endfor %}
						</ul>
						{% endnavList %}`,
					},
					{
						Title: "一二级导航+三级栏目",
						Code: `<ul>
						{% navList navList with typeId=1 %}
						{%- for item in navList %}
						<li>
							<a href="{{ item.Link }}">{{item.Title}}</a>
							{%- if item.NavList %}
							<ul class="nav-menu-child">
								{%- for inner in item.NavList %}
								<li>
									<a href="{{ inner.Link }}">{{inner.Title}}</a>
									{% if inner.PageId > 0 %}
										{% categoryList categories with parentId=inner.PageId %}
										{% if categories %}
										<ul>
											{% for item in categories %}
											<li>
												<a href="{{ item.Link }}">{{item.Title}}</a>
											</li>
											{% endfor %}
										</ul>
										{% endif %}
										{% endcategoryList %}
									{% endif %}
								</li>
								{% endfor %}
							</ul>
							{% endif %}
						</li>
						{% endfor %}
						{% endnavList %}
					</ul>`,
					},
					{
						Title: "一二级导航+三级文档",
						Code: `<ul>
						{% navList navList with typeId=1 %}
						{%- for item in navList %}
						<li>
							<a href="{{ item.Link }}">{{item.Title}}</a>
							{%- if item.NavList %}
							<ul class="nav-menu-child">
								{%- for inner in item.NavList %}
								<li>
									<a href="{{ inner.Link }}">{{inner.Title}}</a>
									{% if inner.PageId > 0 %}
										{% archiveList archives with type="list" categoryId=inner.PageId limit="8" %}
										{% if archives %}
										<ul class="nav-menu-child-child">
											{% for item in archives %}
											<li><a href="{{item.Link}}">{{item.Title}}</a></li>
											{% endfor %}
										</ul>
										{% endif %}
										{% endarchiveList %}
									{% endif %}
								</li>
								{% endfor %}
							</ul>
							{% endif %}
						</li>
						{% endfor %}
						{% endnavList %}
					</ul>`,
					},
				},
			},
			{
				Title: "面包屑导航标签",
				Link:  "https://www.anqicms.com/manual-normal/87.html",
				Docs: []response.DesignDoc{
					{
						Title: "显示标题",
						Code: `{% breadcrumb crumbs %}
						<ul>
							{% for item in crumbs %}
								<li><a href="{{item.Link}}">{{item.Name}}</a></li>        
							{% endfor %}
						</ul>
						{% endbreadcrumb %}`,
					},
					{
						Title: "不显示标题",
						Code: `{% breadcrumb crumbs with title=false %}
						<ul>
							{% for item in crumbs %}
								<li><a href="{{item.Link}}">{{item.Name}}</a></li>        
							{% endfor %}
						</ul>
						{% endbreadcrumb %}`,
					},
				},
			},
			{
				Title: "统计代码标签",
				Link:  "https://www.anqicms.com/manual-normal/91.html",
				Code:  `{{- pluginJsCode|safe }}`,
			},
			{
				Title: "首页Banner标签",
				Link:  "https://www.anqicms.com/manual-normal/3317.html",
				Code: `{% bannerList banners %}
					{% for item in banners %}
					<a href="{{item.Link}}" target="_blank">
						<img src="{{item.Logo}}" alt="{{item.Alt}}" />
						<h5>{{item.Title}}</h5>
					</a>
					{% endfor %}
				{% endbannerList %}`,
			},
		},
	},
	{
		Title: "分类页面标签",
		Type:  "tag",
		Docs: []response.DesignDoc{
			{
				Title: "分类列表标签",
				Link:  "https://www.anqicms.com/manual-category/77.html",
				Docs: []response.DesignDoc{
					{
						Title: "一级分类",
						Code: `{% categoryList categories with moduleId="1" parentId="0" %}
						<ul>
							{% for item in categories %}
							<li>
								<a href="{{ item.Link }}">{{item.Title}}</a>
							</li>
							{% endfor %}
						</ul>
						{% endcategoryList %}`,
					},
					{
						Title: "多级分类嵌套",
						Code: `{% categoryList categories with moduleId="1" parentId="0" %}
						{#一级分类#}
						<ul>
							{% for item in categories %}
							<li>
								<a href="{{ item.Link }}">{{item.Title}}</a>
								<div>
									{% categoryList subCategories with parentId=item.Id %}
									{#二级分类#}
									<ul>
										{% for inner1 in subCategories %}
										<li>
											<a href="{{ inner1.Link }}">{{inner1.Title}}</a>
											<div>
												{% categoryList subCategories2 with parentId=inner1.Id %}
												{#三级分类#}
												<ul>
													{% for inner2 in subCategories2 %}
													<li>
														<a href="{{ inner2.Link }}">{{inner2.Title}}</a>
													</li>
													{% endfor %}
												</ul>
												{% endcategoryList %}
											</div>
										</li>
										{% endfor %}
									</ul>
									{% endcategoryList %}
								</div>
							</li>
							{% endfor %}
						</ul>
						{% endcategoryList %}`,
					},
					{
						Title:   "文章分类+文档组合",
						Content: "如需调用其他模型的分类，只需要更改moduleId=\"1\"为其它值即可",
						Code: `{% categoryList categories with moduleId="1" parentId="0" %}
						<div>
							{% for item in categories %}
							<div>
								<h3><a href="{{ item.Link }}">{{item.Title}}</a></h3>
								<ul>
									{% archiveList archives with type="list" categoryId=item.Id limit="6" %}
									{% for archive in archives %}
									<li>
										<a href="{{archive.Link}}">
											<h5>{{archive.Title}}</h5>
											<div>{{archive.Description}}</div>
											<div>
												<span>{{stampToDate(archive.CreatedTime, "2006-01-02")}}</span>
												<span>{{archive.Views}} 阅读</span>
											</div>
										</a>
										{% if archive.Thumb %}
										<a href="{{archive.Link}}">
											<img alt="{{archive.Title}}" src="{{archive.Thumb}}">
										</a>
										{% endif %}
									</li>
									{% empty %}
									<li>
										该列表没有任何内容
									</li>
									{% endfor %}
								{% endarchiveList %}
								</ul>
							</div>
							{% endfor %}
						</div>
						{% endcategoryList %}`,
					},
				},
			},
			{
				Title: "分类详情标签",
				Link:  "https://www.anqicms.com/manual-category/78.html",
				Docs: []response.DesignDoc{
					{
						Title: "分类ID",
						Code:  `{% categoryDetail with name="Id" %}`,
					},
					{
						Title: "分类标题",
						Code:  `{% categoryDetail with name="Title" %}`,
					},
					{
						Title: "分类链接",
						Code:  `{% categoryDetail with name="Link" %}`,
					},
					{
						Title: "分类描述",
						Code:  `{% categoryDetail with name="Description" %}`,
					},
					{
						Title: "分类内容",
						Code:  `{% categoryDetail with name="Content" %}`,
					},
					{
						Title: "上级分类ID",
						Code:  `{% categoryDetail with name="ParentId" %}`,
					},
					{
						Title: "分类缩略图大图",
						Code:  `{% categoryDetail with name="Logo" %}`,
					},
					{
						Title: "分类缩略图",
						Code:  `{% categoryDetail with name="Thumb" %}`,
					},
					{
						Title: "分类Banner图",
						Code: `{% categoryDetail categoryImages with name="Images" %}
						<ul>
						{% for item in categoryImages %}
						  <li>
							<img src="{{item}}" alt="{% categoryDetail with name="Title" %}" />
						  </li>
						{% endfor %}
						</ul>`,
					},
				},
			},
			{
				Title: "单页列表标签",
				Link:  "https://www.anqicms.com/manual-category/83.html",
				Code: `<ul>
				{% pageList pages %}
					{% for item in pages %}
					<li>
						<a href="{{ item.Link }}">{{item.Title}}</a>
					</li>
					{% endfor %}
				{% endpageList %}
				</ul>`,
			},
			{
				Title: "单页详情标签",
				Link:  "https://www.anqicms.com/manual-category/84.html",
				Docs: []response.DesignDoc{
					{
						Title: "单页ID",
						Code:  `{% pageDetail with name="Id" %}`,
					},
					{
						Title: "单页标题",
						Code:  `{% pageDetail with name="Title" %}`,
					},
					{
						Title: "单页链接",
						Code:  `{% pageDetail with name="Link" %}`,
					},
					{
						Title: "单页描述",
						Code:  `{% pageDetail with name="Description" %}`,
					},
					{
						Title: "单页内容",
						Code:  `{% pageDetail with name="Content" %}`,
					},
					{
						Title: "单页缩略图大图",
						Code:  `{% pageDetail with name="Logo" %}`,
					},
					{
						Title: "单页缩略图",
						Code:  `{% pageDetail with name="Thumb" %}`,
					},
					{
						Title: "单页Banner图",
						Code: `{% pageDetail pageImages with name="Images" %}
						<ul>
						{% for item in pageImages %}
						  <li>
							<img src="{{item}}" alt="{% pageDetail with name="Title" %}" />
						  </li>
						{% endfor %}
						</ul>`,
					},
				},
			},
			{
				Title: "文档模型详情标签",
				Link:  "https://www.anqicms.com/manual-normal/3489.html",
				Docs: []response.DesignDoc{
					{
						Title: "模型ID",
						Code:  `{% moduleDetail with name="Id" %}`,
					},
					{
						Title: "模型标题",
						Code:  `{% moduleDetail with name="Title" %}`,
					},
					{
						Title: "模型链接",
						Code:  `{% moduleDetail with name="Link" %}`,
					},
					{
						Title: "模型表名",
						Code:  `{% moduleDetail with name="TableName" %}`,
					},
				},
			},
		},
	},
	{
		Title: "文档标签",
		Type:  "tag",
		Docs: []response.DesignDoc{
			{
				Title: "文档列表标签",
				Link:  "https://www.anqicms.com/manual-archive/79.html",
				Docs: []response.DesignDoc{
					{
						Title: "文档列表展示",
						Options: []response.DocOption{
							{
								Title: "文档ID",
								Code:  `{{item.Id}}`,
							},
							{
								Title: "文档标题",
								Code:  `{{item.Title}}`,
							},
							{
								Title: "文档关键词",
								Code:  `{{item.Keywords}}`,
							},
							{
								Title: "文档描述",
								Code:  `{{item.Description}}`,
							},
							{
								Title: "文档链接",
								Code:  `{{item.Link}}`,
							},
							{
								Title: "浏览量",
								Code:  `{{item.Views}}`,
							},
							{
								Title: "发布日期",
								Code:  `{{stampToDate(item.CreatedTime, "2006-01-02")}}`,
							},
							{
								Title: "更新日期",
								Code:  `{{stampToDate(item.UpdatedTime, "2006-01-02")}}`,
							},
							{
								Title: "封面首图",
								Code:  `<img src="{{item.Logo}}" />`,
							},
							{
								Title: "缩略图",
								Code:  `<img src="{{item.Thumb}}" />`,
							},
							{
								Title: "封面组图",
								Code: `<ul>
								{% for inner in item.Images %}
								<li>
									<img src="{{inner}}" alt="{{item.Title}}" />
								</li>
								{% endfor %}
								</ul>`,
							},
							{
								Title: "文档标签",
								Code: `{% tagList tags with itemId=item.Id limit="10" %}
								{% for item in tags %}
								<a href="{{item.Link}}">{{item.Title}}</a>
								{% endfor %}
								{% endtagList %}`,
							},
							{
								Title: "自定义字段",
								Code: `{% archiveParams params with id=item.Id %}
								<div>
									{% for item in params %}
									<div>
										<span>{{item.Name}}：</span>
										<span>{{item.Value}}</span>
									</div>
									{% endfor %}
								</div>
								{% endarchiveParams %}`,
							},
						},
						Code: `<ul>
						{% archiveList archives with type="list" limit="10" %}
							{% for item in archives %}
							<li>
								<a href="{{item.Link}}">{{item.Title}}</a>
							</li>
							{% empty %}
							<li>
								该列表没有任何内容
							</li>
							{% endfor %}
						{% endarchiveList %}
						</ul>`,
					},
					{
						Title: "分页文档展示",
						Options: []response.DocOption{
							{
								Title: "文档ID",
								Code:  `{{item.Id}}`,
							},
							{
								Title: "文档标题",
								Code:  `{{item.Title}}`,
							},
							{
								Title: "文档关键词",
								Code:  `{{item.Keywords}}`,
							},
							{
								Title: "文档描述",
								Code:  `{{item.Description}}`,
							},
							{
								Title: "文档链接",
								Code:  `{{item.Link}}`,
							},
							{
								Title: "浏览量",
								Code:  `{{item.Views}}`,
							},
							{
								Title: "发布日期",
								Code:  `{{stampToDate(item.CreatedTime, "2006-01-02")}}`,
							},
							{
								Title: "更新日期",
								Code:  `{{stampToDate(item.UpdatedTime, "2006-01-02")}}`,
							},
							{
								Title: "封面首图",
								Code:  `<img src="{{item.Logo}}" />`,
							},
							{
								Title: "缩略图",
								Code:  `<img src="{{item.Thumb}}" />`,
							},
							{
								Title: "封面组图",
								Code: `<ul>
								{% for inner in item.Images %}
								<li>
									<img src="{{inner}}" alt="{{item.Title}}" />
								</li>
								{% endfor %}
								</ul>`,
							},
							{
								Title: "文档标签",
								Code: `{% tagList tags with itemId=item.Id limit="10" %}
								{% for item in tags %}
								<a href="{{item.Link}}">{{item.Title}}</a>
								{% endfor %}
								{% endtagList %}`,
							},
							{
								Title: "自定义字段",
								Code: `{% archiveParams params with id=item.Id %}
								<div>
									{% for item in params %}
									<div>
										<span>{{item.Name}}：</span>
										<span>{{item.Value}}</span>
									</div>
									{% endfor %}
								</div>
								{% endarchiveParams %}`,
							},
						},
						Code: `<ul>
						{% archiveList archives with type="page" q="seo" limit="10" %}
							{% for item in archives %}
							<li>
								<a href="{{item.Link}}">
									<h5>{{item.Title}}</h5>
									<div>{{item.Description}}</div>
									<div>
										<span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
										<span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
										<span>{{item.Views}} 阅读</span>
									</div>
								</a>
								{% if item.Thumb %}
								<a href="{{item.Link}}">
									<img alt="{{item.Title}}" src="{{item.Thumb}}">
								</a>
								{% endif %}
							</li>
							{% empty %}
							<li>
								该列表没有任何内容
							</li>
							{% endfor %}
						{% endarchiveList %}
						</ul>
							{# 分页代码 #}
							<div>
								{% pagination pages with show="5" %}
									{# 首页 #}
									<a class="{% if pages.FirstPage.IsCurrent %}active{% endif %}" href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a>
									{# 上一页 #}
									{% if pages.PrevPage %}
									<a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a>
									{% endif %}
									{# 中间多页 #}
									{% for item in pages.Pages %}
									<a class="{% if item.IsCurrent %}active{% endif %}" href="{{item.Link}}">{{item.Name}}</a>
									{% endfor %}
									{# 下一页 #}
									{% if pages.NextPage %}
									<a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a>
									{% endif %}
									{# 尾页 #}
									<a class="{% if pages.LastPage.IsCurrent %}active{% endif %}" href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a>
								{% endpagination %}
							</div>`,
					},
				},
			},
			{
				Title: "文档详情标签",
				Link:  "https://www.anqicms.com/manual-archive/80.html",
				Docs: []response.DesignDoc{
					{
						Title: "文档ID",
						Code:  `{% archiveDetail with name="Id" %}`,
					},
					{
						Title: "文档标题",
						Code:  `{% archiveDetail with name="Title" %}`,
					},
					{
						Title: "文档关键词",
						Code:  `{% archiveDetail with name="Keywords" %}`,
					},
					{
						Title: "文档链接",
						Code:  `{% archiveDetail with name="Link" %}`,
					},
					{
						Title: "文档描述",
						Code:  `{% archiveDetail with name="Description" %}`,
					},
					{
						Title: "文档内容",
						Code:  `{% archiveDetail archiveContent with name="Content" %}{{archiveContent|safe}}`,
					},
					{
						Title: "浏览量",
						Code:  `{% archiveDetail with name="Views" %}`,
					},
					{
						Title: "发布日期",
						Code:  `{% archiveDetail with name="CreatedTime" format="2006-01-02 15:04" %}`,
					},
					{
						Title: "更新日期",
						Code:  `{% archiveDetail with name="UpdatedTime" format="2006-01-02 15:04" %}`,
					},
					{
						Title: "封面首图",
						Code:  `<img src="{% archiveDetail with name="Logo" %}" alt=""/>`,
					},
					{
						Title: "缩略图",
						Code:  `<img src="{% archiveDetail with name="Thumb" %}" alt=""/>`,
					},
					{
						Title: "封面组图",
						Code: `{% archiveDetail archiveImages with name="Images" %}
						{% for item in archiveImages %}
							<img src="{{item}}" alt=""/>
						{% endfor %}`,
					},
					{
						Title: "文档分类",
						Code:  `<a href="{% categoryDetail with name="Link" %}">{% categoryDetail with name="Title" %}</a>`,
					},
					{
						Title: "文档标签",
						Code: `{% tagList tags with limit="10" %}
								{% for item in tags %}
								<a href="{{item.Link}}">{{item.Title}}</a>
								{% endfor %}
								{% endtagList %}`,
					},
					{
						Title: "自定义字段",
						Code: `{% archiveParams params %}
								<div>
									{% for item in params %}
									<div>
										<span>{{item.Name}}：</span>
										<span>{{item.Value}}</span>
									</div>
									{% endfor %}
								</div>
								{% endarchiveParams %}`,
					},
				},
			},
			{
				Title: "上一篇文档标签",
				Link:  "https://www.anqicms.com/manual-archive/88.html",
				Code: `{% prevArchive prev %}
				上一篇：
				{% if prev %}
				  <a href="{{prev.Link}}">{{prev.Title}}</a>
				{% else %}
				  没有了
				{% endif %}
				{% endprevArchive %}`,
			},
			{
				Title: "下一篇文档标签",
				Link:  "https://www.anqicms.com/manual-archive/89.html",
				Code: `{% nextArchive next %}
				下一篇：
				{% if next %}
				  <a href="{{next.Link}}">{{next.Title}}</a>
				{% else %}
				  没有了
				{% endif %}
				{% endnextArchive %}`,
			},
			{
				Title: "相关文档标签",
				Link:  "https://www.anqicms.com/manual-archive/92.html",
				Code: `<div>
				{% archiveList archives with type="related" limit="10" %}
					{% for item in archives %}
					<li>
						<a href="{{item.Link}}">
							<h5>{{item.Title}}</h5>
							<div>{{item.Description}}</div>
							<div>
								<span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
								<span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
								<span>{{item.Views}} 阅读</span>
							</div>
						</a>
						{% if item.Thumb %}
						<a href="{{item.Link}}">
							<img alt="{{item.Title}}" src="{{item.Thumb}}">
						</a>
						{% endif %}
					</li>
					{% empty %}
					<li>
						该列表没有任何内容
					</li>
					{% endfor %}
				{% endarchiveList %}
				</div>`,
			},
			{
				Title: "文档参数标签",
				Link:  "https://www.anqicms.com/manual-archive/95.html",
				Code: `<div>
					{% archiveParams params %}
					{% for item in params %}
					<div>
						<span>{{item.Name}}：</span>
						<span>{{item.Value}}</span>
					</div>
					{% endfor %}
					{% endarchiveParams %}
				</div>`,
			},
			{
				Title: "文档参数筛选标签",
				Link:  "https://www.anqicms.com/manual-archive/96.html",
				Code: `<div>
					{% archiveFilters filters with moduleId="1" allText="不限" %}
						{% for item in filters %}
						<ul>
							<li>{{item.Name}}: </li>
							{% for val in item.Items %}
							<li class="{% if val.IsCurrent %}active{% endif %}"><a href="{{val.Link}}">{{val.Label}}</a></li>
							{% endfor %}
						</ul>
					{% endfor %}
					{% endarchiveFilters %}
				</div>`,
			},
		},
	},
	{
		Title: "文档Tag标签",
		Type:  "tag",
		Docs: []response.DesignDoc{
			{
				Title: "文档Tag列表标签",
				Link:  "https://www.anqicms.com/manual-tag/81.html",
				Docs: []response.DesignDoc{
					{
						Title: "普通Tag列表",
						Code: `{% tagList tags with limit="10" %}
						{% for item in tags %}
						<a href="{{item.Link}}">{{item.Title}}</a>
						{% endfor %}
						{% endtagList %}`,
					},
					{
						Title: "带分页Tag列表",
						Code: `<div>
							{% tagList tags with type="page" limit="20" %}
							<ul>
							{% for item in tags %}
							<li>
								<a href="{{item.Link}}">
									<h5>{{item.Title}}</h5>
									<div>{{item.Description}}</div>
								</a>
							</li>
							{% empty %}
							<liå>
								该列表没有任何内容
							</li>
							{% endfor %}
							</ul>
							{% endtagList %}
						</div>
						
						{# 分页代码 #}
						  <div>
							{% pagination pages with show="5" %}
								{# 首页 #}
								<a class="{% if pages.FirstPage.IsCurrent %}active{% endif %}" href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a>
								{# 上一页 #}
								{% if pages.PrevPage %}
								<a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a>
								{% endif %}
								{# 中间多页 #}
								{% for item in pages.Pages %}
								<a class="{% if item.IsCurrent %}active{% endif %}" href="{{item.Link}}">{{item.Name}}</a>
								{% endfor %}
								{# 下一页 #}
								{% if pages.NextPage %}
								<a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a>
								{% endif %}
								{# 尾页 #}
								<a class="{% if pages.LastPage.IsCurrent %}active{% endif %}" href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a>
							{% endpagination %}
						  </div>
						</div>`,
					},
				},
			},
			{
				Title: "Tag文档列表标签",
				Link:  "https://www.anqicms.com/manual-tag/82.html",
				Code: `<ul>
					{% tagDataList archives with type="page" limit="10" %}
						{% for item in archives %}
						<li>
							<a href="{{item.Link}}">
								<h5>{{item.Title}}</h5>
								<div>{{item.Description}}</div>
								<div>
									<span>{% categoryDetail with name="Title" id=item.CategoryId %}</span>
									<span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
									<span>{{item.Views}} 阅读</span>
								</div>
							</a>
							{% if item.Thumb %}
							<a href="{{item.Link}}">
								<img alt="{{item.Title}}" src="{{item.Thumb}}">
							</a>
							{% endif %}
						</li>
						{% empty %}
						<li>
							该列表没有任何内容
						</li>
						{% endfor %}
					{% endtagDataList %}
					</ul>
						{# 分页代码 #}
						<div>
							{% pagination pages with show="5" %}
								{# 首页 #}
								<a class="{% if pages.FirstPage.IsCurrent %}active{% endif %}" href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a>
								{# 上一页 #}
								{% if pages.PrevPage %}
								<a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a>
								{% endif %}
								{# 中间多页 #}
								{% for item in pages.Pages %}
								<a class="{% if item.IsCurrent %}active{% endif %}" href="{{item.Link}}">{{item.Name}}</a>
								{% endfor %}
								{# 下一页 #}
								{% if pages.NextPage %}
								<a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a>
								{% endif %}
								{# 尾页 #}
								<a class="{% if pages.LastPage.IsCurrent %}active{% endif %}" href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a>
							{% endpagination %}
						</div>`,
			},
			{
				Title: "Tag详情标签",
				Link:  "https://www.anqicms.com/manual-tag/90.html",
				Docs: []response.DesignDoc{
					{
						Title: "TagID",
						Code:  `{% tagDetail with name="Id" %}`,
					},
					{
						Title: "Tag标题",
						Code:  `{% tagDetail with name="Title" %}`,
					},
					{
						Title: "Tag链接",
						Code:  `{% tagDetail with name="Link" %}`,
					},
					{
						Title: "Tag描述",
						Code:  `{% tagDetail with name="Description" %}`,
					},
					{
						Title: "Tag索引字母",
						Code:  `{% tagDetail with name="FirstLetter" %}`,
					},
				},
			},
		},
	},
	{
		Title: "其他标签",
		Type:  "tag",
		Docs: []response.DesignDoc{
			{
				Title: "评论标列表签",
				Link:  "https://www.anqicms.com/manual-other/85.html",
				Docs: []response.DesignDoc{
					{
						Title: "常规评论列表",
						Code: `<div>
						{% commentList comments with archiveId=archive.Id type="list" limit="6" %}
							{% for item in comments %}
							<div>
							  <div>
								<span>
								  {% if item.Status != 1 %}
								  审核中：{{item.UserName|truncatechars:6}}
								  {% else %}
								  {{item.UserName}}
								  {% endif %}
								</span>
								{% if item.Parent %}
								<span>回复</span>
								<span>
								  {% if item.Status != 1 %}
								  审核中：{{item.Parent.UserName|truncatechars:6}}
								  {% else %}
								  {{item.Parent.UserName}}
								  {% endif %}
								</span>
								{% endif %}
								<span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
							  </div>
							  <div>
								{% if item.Parent %}
								<blockquote>
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
								<a class="item" data-id="praise">赞(<span class="vote-count">{{item.VoteCount}}</span>)</a>
								<a class="item" data-id=reply>回复</a>
							  </div>
							</div>
							{% endfor %}
						{% endcommentList %}
						</div>`,
					},
					{
						Title: "分页评论列表",
						Code: `<div>
						{% commentList comments with archiveId=archive.Id type="page" limit="10" %}
							{% for item in comments %}
							<div>
							  <div>
								<span>
								  {% if item.Status != 1 %}
								  审核中：{{item.UserName|truncatechars:6}}
								  {% else %}
								  {{item.UserName}}
								  {% endif %}
								</span>
								{% if item.Parent %}
								<span>回复</span>
								<span>
								  {% if item.Status != 1 %}
								  审核中：{{item.Parent.UserName|truncatechars:6}}
								  {% else %}
								  {{item.Parent.UserName}}
								  {% endif %}
								</span>
								{% endif %}
								<span>{{stampToDate(item.CreatedTime, "2006-01-02")}}</span>
							  </div>
							  <div>
								{% if item.Parent %}
								<blockquote>
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
								<a class="item" data-id="praise">赞(<span class="vote-count">{{item.VoteCount}}</span>)</a>
								<a class="item" data-id=reply>回复</a>
							  </div>
							</div>
							{% endfor %}
						{% endcommentList %}
						</div>
						
						<div>
							{% pagination pages with show="5" %}
							<ul>
								<li>总数：{{pages.TotalItems}}条，总共：{{pages.TotalPages}}页，当前第{{pages.CurrentPage}}页</li>
								<li class="{% if pages.FirstPage.IsCurrent %}active{% endif %}"><a href="{{pages.FirstPage.Link}}">{{pages.FirstPage.Name}}</a></li>
								{% if pages.PrevPage %}
									<li><a href="{{pages.PrevPage.Link}}">{{pages.PrevPage.Name}}</a></li>
								{% endif %}
								{% for item in pages.Pages %}
									<li class="{% if item.IsCurrent %}active{% endif %}"><a href="{{item.Link}}">{{item.Name}}</a></li>
								{% endfor %}
								{% if pages.NextPage %}
									<li><a href="{{pages.NextPage.Link}}">{{pages.NextPage.Name}}</a></li>
								{% endif %}
								<li class="{% if pages.LastPage.IsCurrent %}active{% endif %}"><a href="{{pages.LastPage.Link}}">{{pages.LastPage.Name}}</a></li>
							</ul>
							{% endpagination %}
						</div>`,
					},
					{
						Title: "评论表单提交",
						Code: `<form method="post" action="/comment/publish">
						  <input type="hidden" name="return" value="html">
						  <input type="hidden" name="archive_id" value="{% archiveDetail with name="Id" %}">
						  <div>
							<label>用户名</label>
							<div>
							  <input type="text" name="user_name" required lay-verify="required" placeholder="请填写您的昵称" autocomplete="off">
							</div>
						  </div>
						  <div>
							<label>评论内容</label>
							<div>
							  <textarea name="content" placeholder="" id="comment-content-field" rows="5"></textarea>
							</div>
						  </div>
						  <div>
							<div>
							  <button type="submit">提交评论</button>
							  <button type="reset">重置</button>
							</div>
						  </div>
						</form>`,
					},
				},
			},
			{
				Title: "留言表单标签",
				Link:  "https://www.anqicms.com/manual-other/86.html",
				Docs: []response.DesignDoc{
					{
						Title:   "默认留言表单",
						Content: "通过下面的代码，可以循环输出所有的设置的字段",
						Code: `<form method="post" action="/guestbook.html">
						{% guestbook fields %}
							{% for item in fields %}
							<div>
								<label>{{item.Name}}</label>
								<div>
									{% if item.Type == "text" || item.Type == "number" %}
									<input type="{{item.Type}}" name="{{item.FieldName}}" {% if item.Required %}required lay-verify="required"{% endif %} placeholder="{{item.Content}}" autocomplete="off">
									{% elif item.Type == "textarea" %}
									<textarea name="{{item.FieldName}}" {% if item.Required %}required lay-verify="required"{% endif %} placeholder="{{item.Content}}" rows="5"></textarea>
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
							<div>
								<div>
									<button type="submit">提交留言</button>
									<button type="reset">重置</button>
								</div>
							</div>
						{% endguestbook %}
						</form>`,
					},
					{
						Title:   "自定义留言表单字段",
						Content: "如果你想自定义表单显示，你也可以使用常规的input来组织显示",
						Code: `<form method="post" action="/guestbook.html">
						  <input type="hidden" name="return" value="html">
						  <div>
							<label>用户名</label>
							<div>
							  <input type="text" name="user_name" required lay-verify="required" placeholder="请填写您的昵称" autocomplete="off">
							</div>
						  </div>
						  <div>
							<label>联系方式</label>
							<div>
							  <input type="text" name="contact" required lay-verify="required" placeholder="请填写您的手机号或微信" autocomplete="off">
							</div>
						  </div>
						  <div>
							<label>留言内容内容</label>
							<div>
							  <textarea name="content" placeholder="" id="comment-content-field" rows="5"></textarea>
							</div>
						  </div>
							<div>
								<div>
									<button type="submit">提交留言</button>
									<button type="reset">重置</button>
								</div>
							</div>
						</form>`,
					},
				},
			},
			{
				Title: "分页标签",
				Link:  "https://www.anqicms.com/manual-other/94.html",
				Code: `<div class="pagination">
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
				</div>`,
			},
			{
				Title: "友情链接标签",
				Link:  "https://www.anqicms.com/manual-other/97.html",
				Code: `{% linkList friendLinks %}
				{% if friendLinks %}
				<div>
					<span>友情链接：</span>
					{% for item in friendLinks %}
					<a href="{{item.Link}}" {% if item.Nofollow == 1 %} rel="nofollow"{% endif %} target="_blank">{{item.Title}}</a>
					{% endfor %}
				</div>
				{% endif %}
				{% endlinkList %}`,
			},
			{
				Title: "留言验证码使用标签",
				Link:  "https://www.anqicms.com/manual-other/139.html",
				Code: `<div style="display: flex; clear: both">
					<input type="hidden" name="captcha_id" id="captcha_id">
					<input type="text" name="captcha" required placeholder="请填写验证码" class="layui-input" style="flex: 1">
					<img src="" id="get-captcha" style="width: 150px;height: 56px;cursor: pointer;" />
					<script>
					  document.getElementById('get-captcha').addEventListener("click", function (e) {
						fetch('/api/captcha')
								.then(response => {
								  return response.json()
								})
								.then(res => {
								  document.getElementById('captcha_id').setAttribute("value", res.data.captcha_id)
								  document.getElementById('get-captcha').setAttribute("src", res.data.captcha)
								}).catch(err =>{console.log(err)})
					  });
					  document.getElementById('get-captcha').click();
					</script>
				  </div>`,
			},
		},
	},
	{
		Title: "Filter过滤器",
		Type:  "filter",
		Docs: []response.DesignDoc{
			{
				Title:   "判断包含某个关键词",
				Link:    "https://www.anqicms.com/manual-filter/250.html",
				Content: "contain 过滤器可以判断某个关键词是否包含在一行字符串、数组（slice）、键值对（map）、结构体（struct）中，结果将会返回一个布尔值（bool）",
				Code:    `{{obj|contain:"关键词"}}`,
			},
			{
				Title:   "删除首尾空格/关键词",
				Link:    "https://www.anqicms.com/manual-filter/251.html",
				Content: `trim、trimLeft、trimRight 过滤器可以分别删除字符串首尾空格、特定字符。`,
				Code:    `{{obj|trim}}`,
			},
			{
				Title:   "计算关键词出现次数",
				Link:    "https://www.anqicms.com/manual-filter/252.html",
				Content: `count 过滤器可以计算某个关键词在一行字符串或数组（array/slice）中出现的次数。`,
				Code:    `{{obj|count:"关键词"}}`,
			},
			{
				Title:   "获取关键词出现位置",
				Link:    "https://www.anqicms.com/manual-filter/254.html",
				Content: `index 过滤器可以计算某个关键词在一行字符串或数组（array/slice）中出现的位置。如果字符串中包含多个需要查找的关键词，则index返回的是首次出现的位置。如果没有找到，则返回-1。注意：如果字符串中有中文，则计算位置的时候，一个中文算3个位置。`,
				Code:    `{{obj|index:"关键词"}}`,
			},
			{
				Title:   "替换关键词",
				Link:    "https://www.anqicms.com/manual-filter/256.html",
				Content: `replace 过滤器可以将字符串中的旧的词old替换词新的词new，返回替换后的新字符串。如果 old 为空，它将在字符串的开头和每个 UTF-8 序列之后进行匹配。如果new为空，则移除old。`,
				Code:    `{{obj|replace:"old,new"}}`,
			},
			{
				Title:   "重复输出多次",
				Link:    "https://www.anqicms.com/manual-filter/257.html",
				Content: `repeat 过滤器可以将一个字符串按指定次数重复。`,
				Code:    `{{obj|repeat:次数}}`,
			},
			{
				Title:   "数字/字符串相加",
				Link:    "https://www.anqicms.com/manual-filter/259.html",
				Content: `add 过滤器可以将怎么将两个数字、字符串相加。add 可以将整数、浮点数、字符串混合相加。也就是在你进行相加计算的时候，可以忽略他们的类型，在自动转换失败的时候，则会忽略相加的内容。`,
				Code:    `{{ obj|add:obj2 }}`,
			},
			{
				Title:   "首字母转大写",
				Link:    "https://www.anqicms.com/manual-filter/261.html",
				Content: `capfirst 过滤器可以将英文字符串第一个字母转换为大写。只有英文字母会被转换。`,
				Code:    `{{ obj|capfirst }}`,
			},
			{
				Title:   "英文转大写",
				Link:    "https://www.anqicms.com/manual-filter/261.html",
				Content: `upper 过滤器可以将英文字符串中所有的字母转换成大写。`,
				Code:    `{{ obj|upper }}`,
			},
			{
				Title:   "英文转小写",
				Link:    "https://www.anqicms.com/manual-filter/261.html",
				Content: `lower 过滤器可以将英文字符串中所有的字母转换成小写。`,
				Code:    `{{ obj|lower }}`,
			},
			{
				Title:   "每个单词首字母转大写",
				Link:    "https://www.anqicms.com/manual-filter/261.html",
				Content: `title 过滤器可以将英文字符串中所有的单词的首字母转换成大写。`,
				Code:    `{{ obj|title }}`,
			},
			{
				Title:   "默认值设置",
				Link:    "https://www.anqicms.com/manual-filter/265.html",
				Content: `default 过滤器可以在如果数字或字符串、对象没有值的时候给数字或字符串设置默认值。default_if_none 过滤器可以判断指针类型的对象是否为空，如果为空，则设置默认值。`,
				Code:    `{{ obj|default:"默认值" }}`,
			},
			{
				Title:   "HTML代码不转义",
				Link:    "https://www.anqicms.com/manual-filter/280.html",
				Content: `safe 过滤器可以取消模板输出的默认转义属性，让直接输出html代码到界面，让浏览器解析HTML代码。一般用在富文本输出中，如显示文章详情等情况下。注意：使用 safe 过滤器，默认认为你的输出是安全的，它不会对特殊字符进行转义，因此如果代码中包含有xss注入等问题情况，它也会原样输出。请注意防范风险。`,
				Code:    `{{ obj|safe }}`,
			},
			{
				Title:   "字符串或数组第一个值",
				Link:    "https://www.anqicms.com/manual-filter/269.html",
				Content: `first 过滤器可以获得字符串第一个字符或数组第一个值。如果原字符串、数组为空，什么也不返回。如果字符串是中文，则返回第一个汉字。`,
				Code:    `{{ obj|first }}`,
			},
			{
				Title:   "字符串或数组最后一个值",
				Link:    "https://www.anqicms.com/manual-filter/269.html",
				Content: `last 过滤器可以获取字符串最后一个字符或数组最后一个值。如果原字符串、数组为空，什么也不返回。如果字符串是中文，则返回最后一个汉字。`,
				Code:    `{{ obj|last }}`,
			},
			{
				Title:   "保留指定位数小数点",
				Link:    "https://www.anqicms.com/manual-filter/270.html",
				Content: `floatformat 过滤器可以将一个浮点数保留2位小数输出。也可以保留指定的其他位数小数点。如保留小数点后3位等。同时支持负数位数，如果设置的是负数，则从最后一位往前推算。`,
				Code:    `{{ obj|floatformat:2 }}`,
			},
			{
				Title:   "获取对象长度",
				Link:    "https://www.anqicms.com/manual-filter/274.html",
				Content: `length 过滤器可以获取字符串、数组、键值对的长度。对于字符串，则计算它的utf8实际字符的数量，一个字母为一个，一个汉字也为1个。数组和键值对则计算它们的索引数量。`,
				Code:    `{{ obj|length }}`,
			},
			{
				Title:   "多行文本转HTML",
				Link:    "https://www.anqicms.com/manual-filter/275.html",
				Content: `linebreaks 过滤器可以将多行文本按换行符转换成html标签。每行开头和结尾采用<p>和</p>包裹，中间有空行则采用 <br/>。还可以使用 linebreaksbr 来进行处理。与 linebreaks不同的地方是，linebreaksbr只是直接将换行符替换成 <br/>，并且不在开头和结尾添加p标签。还可以使用 linenumbers 来给多行文本的每一行进行标号，符号从1开始。如 1.。`,
				Code:    `{{ obj|linebreaks }}`,
			},
			{
				Title:   "移除html代码",
				Link:    "https://www.anqicms.com/manual-filter/279.html",
				Content: `striptags 过滤器可以移除html代码中的所有html标签。removetags 过滤器可以将移除html代码中指定标签。`,
				Code:    `{{ obj|striptags }}`,
			},
			{
				Title:   "任意值格式化成字符串输出",
				Link:    "https://www.anqicms.com/manual-filter/283.html",
				Content: `stringformat 过滤器可以将数字、字符串、数组等任意值按指定格式格式化成字符串输出。`,
				Code:    `{{ obj|stringformat:"#+v" }}`,
			},
			{
				Title:   "字符串截取并添加...",
				Link:    "https://www.anqicms.com/manual-filter/284.html",
				Content: `truncatechars 过滤器可以对字符串进行截取并添加...，该方法会截断单词，指定长度包括...。truncatewords 过滤器可以对字符串按单词进行截取并添加...。`,
				Code:    `{{ obj|truncatechars:50 }}`,
			},
			{
				Title:   "HTML截取并添加...",
				Link:    "https://www.anqicms.com/manual-filter/284.html",
				Content: `truncatechars_html 过滤器可以对html代码进行截取并添加...，该方法会截断单词，指定长度包括...。truncatewords_html 过滤器可以对html代码按单词进行截取并添加...。`,
				Code:    `{{ obj|truncatechars_html:200 }}`,
			},
		},
	},
}
