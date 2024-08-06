package provider

import "kandaoni.com/anqicms/response"

func (w *Website) GetDesignDocs() []response.DesignDocGroup {

	var designDocs = []response.DesignDocGroup{
		{
			Title: w.Tr("TemplateCreationHelp"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("SomeBasicConventions"),
					Link:  "https://www.anqicms.com/help-design/116.html",
				},
				{
					Title: w.Tr("DirectoryAndTemplate"),
					Link:  "https://www.anqicms.com/help-design/117.html",
				},
				{
					Title: w.Tr("LabelsAndUsage"),
					Link:  "https://www.anqicms.com/help-design/118.html",
				},
			},
		},
		{
			Title: w.Tr("CommonLabels"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("SystemSettingsLabels"),
					Link:  "https://www.anqicms.com/manual-normal/73.html",
				},
				{
					Title: w.Tr("ContactPartyTag"),
					Link:  "https://www.anqicms.com/manual-normal/74.html",
				},
				{
					Title: w.Tr("UniversalTdkTag"),
					Link:  "https://www.anqicms.com/manual-normal/75.html",
				},
				{
					Title: w.Tr("NavigationListTag"),
					Link:  "https://www.anqicms.com/manual-normal/76.html",
				},
				{
					Title: w.Tr("BreadcrumbNavigationTag"),
					Link:  "https://www.anqicms.com/manual-normal/87.html",
				},
				{
					Title: w.Tr("StatisticalCodeTag"),
					Link:  "https://www.anqicms.com/manual-normal/91.html",
				},
				{
					Title: w.Tr("HomeBannerTag"),
					Link:  "https://www.anqicms.com/manual-normal/3317.html",
				},
			},
		},
		{
			Title: w.Tr("CategoryPageTag"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("CategoryListTag"),
					Link:  "https://www.anqicms.com/manual-category/77.html",
				},
				{
					Title: w.Tr("CategoryDetailsTag"),
					Link:  "https://www.anqicms.com/manual-category/78.html",
				},
				{
					Title: w.Tr("SinglePageListTag"),
					Link:  "https://www.anqicms.com/manual-category/83.html",
				},
				{
					Title: w.Tr("SinglePageDetailsTag"),
					Link:  "https://www.anqicms.com/manual-category/84.html",
				},
				{
					Title: w.Tr("DocumentModelDetailsTag"),
					Link:  "https://www.anqicms.com/manual-normal/3489.html",
				},
			},
		},
		{
			Title: w.Tr("DocumentTag"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("DocumentListTag"),
					Link:  "https://www.anqicms.com/manual-archive/79.html",
				},
				{
					Title: w.Tr("DocumentDetailsTag"),
					Link:  "https://www.anqicms.com/manual-archive/80.html",
				},
				{
					Title: w.Tr("PreviousDocumentTag"),
					Link:  "https://www.anqicms.com/manual-archive/88.html",
				},
				{
					Title: w.Tr("NextDocumentTag"),
					Link:  "https://www.anqicms.com/manual-archive/89.html",
				},
				{
					Title: w.Tr("RelatedDocumentsTag"),
					Link:  "https://www.anqicms.com/manual-archive/92.html",
				},
				{
					Title: w.Tr("DocumentParameterTag"),
					Link:  "https://www.anqicms.com/manual-archive/95.html",
				},
				{
					Title: w.Tr("DocumentParameterFilterTag"),
					Link:  "https://www.anqicms.com/manual-archive/96.html",
				},
			},
		},
		{
			Title: w.Tr("DocumentTagsTag"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("DocumentTagListTag"),
					Link:  "https://www.anqicms.com/manual-tag/81.html",
				},
				{
					Title: w.Tr("TagDocumentListTag"),
					Link:  "https://www.anqicms.com/manual-tag/82.html",
				},
				{
					Title: w.Tr("TagDetailsTag"),
					Link:  "https://www.anqicms.com/manual-tag/90.html",
				},
			},
		},
		{
			Title: w.Tr("OtherTags"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("CommentTag"),
					Link:  "https://www.anqicms.com/manual-other/85.html",
				},
				{
					Title: w.Tr("MessageTag"),
					Link:  "https://www.anqicms.com/manual-other/86.html",
				},
				{
					Title: w.Tr("PaginationTag"),
					Link:  "https://www.anqicms.com/manual-other/94.html",
				},
				{
					Title: w.Tr("FriendlyLinkTag"),
					Link:  "https://www.anqicms.com/manual-other/97.html",
				},
				{
					Title: w.Tr("MessageVerificationCodeUseTag"),
					Link:  "https://www.anqicms.com/manual-other/139.html",
				},
			},
		},
		{
			Title: w.Tr("GeneralTemplateTag"),
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("OtherAuxiliaryTags"),
					Link:  "https://www.anqicms.com/manual-common/93.html",
				},
				{
					Title: w.Tr("MoreFilters"),
					Link:  "https://www.anqicms.com/manual-common/98.html",
				},
				{
					Title: w.Tr("DefineVariableAssignmentTag"),
					Link:  "https://www.anqicms.com/manual-common/99.html",
				},
				{
					Title: w.Tr("FormatTimestampTag"),
					Link:  "https://www.anqicms.com/manual-common/100.html",
				},
				{
					Title: w.Tr("ForLoopTraversalTag"),
					Link:  "https://www.anqicms.com/manual-common/101.html",
				},
				{
					Title: w.Tr("RemoveLogicalTagOccupiedLines"),
					Link:  "https://www.anqicms.com/manual-common/102.html",
				},
				{
					Title: w.Tr("ArithmeticOperationTag"),
					Link:  "https://www.anqicms.com/manual-common/103.html",
				},
				{
					Title: w.Tr("IfLogicalJudgmentTag"),
					Link:  "https://www.anqicms.com/manual-common/104.html",
				},
			},
		},
	}

	return designDocs
}

func (w *Website) GetDesignTplHelpers() []response.DesignDocGroup {

	var designTplHelpers = []response.DesignDocGroup{
		{
			Title: w.Tr("CommonLabels"),
			Type:  "tag",
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("SystemSettingsLabels"),
					Link:  "https://www.anqicms.com/manual-normal/73.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("WebsiteName"),
							Code:  `{% system with name="SiteName" %}`,
						},
						{
							Title: w.Tr("WebsiteLogo"),
							Code:  `{% system with name="SiteLogo" %}`,
						},
						{
							Title: w.Tr("WebsiteRegistrationNumber"),
							Code:  `{% system with name="SiteIcp" %}`,
						},
						{
							Title: w.Tr("CopyrightContent"),
							Code:  `{% system with name="SiteCopyright" %}`,
						},
						{
							Title: w.Tr("WebsiteHomepageAddress"),
							Code:  `{% system with name="BaseUrl" %}`,
						},
						{
							Title: w.Tr("WebsiteMobileTerminalAddress"),
							Code:  `{% system with name="MobileUrl" %}`,
						},
						{
							Title: w.Tr("TemplateStaticFileAddress"),
							Code:  `{% system with name="TemplateUrl" %}`,
						},
						{
							Title: w.Tr("CustomParameters"),
							Code:  `{% system with name="自定义参数名称" %}`,
						},
						{
							Title: w.Tr("CurrentYear"),
							Code:  `{% now "2006" %}`,
						},
						{
							Title: w.Tr("CurrentDate"),
							Code:  `{% now "2006-01-02" %}`,
						},
					},
				},
				{
					Title: w.Tr("ContactPartyTag"),
					Link:  "https://www.anqicms.com/manual-normal/74.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("ContactPerson"),
							Code:  `{% contact with name="UserName" %}`,
						},
						{
							Title: w.Tr("ContactPhoneNumber"),
							Code:  `{% contact with name="Cellphone" %}`,
						},
						{
							Title: w.Tr("ContactAddress"),
							Code:  `{% contact with name="Address" %}`,
						},
						{
							Title: w.Tr("ContactEmail"),
							Code:  `{% contact with name="Email" %}`,
						},
						{
							Title: w.Tr("ContactWechat"),
							Code:  `{% contact with name="Wechat" %}`,
						},
						{
							Title: w.Tr("WechatQrCode"),
							Code:  `<img src="{% contact with name="Qrcode" %}" />`,
						},
						{
							Title: w.Tr("ContactQq"),
							Code:  `{% contact with name="QQ" %}`,
						},
						{
							Title: w.Tr("ContactWhatsApp"),
							Code:  `{% contact with name="WhatsApp" %}`,
						},
						{
							Title: w.Tr("ContactFacebookEbook"),
							Code:  `{% contact with name="Facebook" %}`,
						},
						{
							Title: w.Tr("ContactTwitter"),
							Code:  `{% contact with name="Twitter" %}`,
						},
						{
							Title: w.Tr("ContactTiktok"),
							Code:  `{% contact with name="Tiktok" %}`,
						},
						{
							Title: w.Tr("ContactPinterest"),
							Code:  `{% contact with name="Pinterest" %}`,
						},
						{
							Title: w.Tr("ContactLinkedin"),
							Code:  `{% contact with name="Linkedin" %}`,
						},
						{
							Title: w.Tr("ContactInstagram"),
							Code:  `{% contact with name="Instagram" %}`,
						},
						{
							Title: w.Tr("ContactYoutube"),
							Code:  `{% contact with name="Youtube" %}`,
						},
						{
							Title: w.Tr("CustomParameters"),
							Code:  `{% contact with name="自定义参数名称" %}`,
						},
					},
				},
				{
					Title: w.Tr("UniversalTdkTag"),
					Link:  "https://www.anqicms.com/manual-normal/75.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("PageTitle"),
							Code:  `{% tdk with name="Title" %}`,
						},
						{
							Title: w.Tr("PageKeywords"),
							Code:  `{% tdk with name="Keywords" %}`,
						},
						{
							Title: w.Tr("PageDescription"),
							Code:  `{% tdk with name="Description" %}`,
						},
						{
							Title: w.Tr("PageStandardLink"),
							Code:  `{% tdk with name="CanonicalUrl" %}`,
						},
					},
				},
				{
					Title: w.Tr("NavigationListTag"),
					Link:  "https://www.anqicms.com/manual-normal/76.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("FirstLevelNavigation"),
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
							Title: w.Tr("FirstAndSecondLevelNavigation"),
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
							Title: w.Tr("FirstAndSecondLevelNavigationThirdLevelColumn"),
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
							Title: w.Tr("FirstAndSecondLevelNavigationThirdLevelDocument"),
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
					Title: w.Tr("BreadcrumbNavigationTag"),
					Link:  "https://www.anqicms.com/manual-normal/87.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("ShowTitle"),
							Code: `{% breadcrumb crumbs %}
						<ul>
							{% for item in crumbs %}
								<li><a href="{{item.Link}}">{{item.Name}}</a></li>        
							{% endfor %}
						</ul>
						{% endbreadcrumb %}`,
						},
						{
							Title: w.Tr("DontShowTitle"),
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
					Title: w.Tr("StatisticalCodeTag"),
					Link:  "https://www.anqicms.com/manual-normal/91.html",
					Code:  `{{- pluginJsCode|safe }}`,
				},
				{
					Title: w.Tr("HomeBannerTag"),
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
			Title: w.Tr("CategoryPageTag"),
			Type:  "tag",
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("CategoryListTag"),
					Link:  "https://www.anqicms.com/manual-category/77.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("FirstLevelClassification"),
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
							Title: w.Tr("MultiLevelClassificationNesting"),
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
							Title:   w.Tr("ArticleClassification+DocumentCombination"),
							Content: w.Tr("IfYouNeedToCallTheClassificationOfOtherModelsJustChangeModuleidEq1ToOtherValues"),
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
					Title: w.Tr("CategoryDetailsTag"),
					Link:  "https://www.anqicms.com/manual-category/78.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("CategoryId"),
							Code:  `{% categoryDetail with name="Id" %}`,
						},
						{
							Title: w.Tr("CategoryTitle"),
							Code:  `{% categoryDetail with name="Title" %}`,
						},
						{
							Title: w.Tr("CategoryLink"),
							Code:  `{% categoryDetail with name="Link" %}`,
						},
						{
							Title: w.Tr("CategoryDescription"),
							Code:  `{% categoryDetail with name="Description" %}`,
						},
						{
							Title: w.Tr("CategoryContent"),
							Code:  `{% categoryDetail with name="Content" %}`,
						},
						{
							Title: w.Tr("ParentCategoryId"),
							Code:  `{% categoryDetail with name="ParentId" %}`,
						},
						{
							Title: w.Tr("CategoryThumbnailImage"),
							Code:  `{% categoryDetail with name="Logo" %}`,
						},
						{
							Title: w.Tr("CategoryThumbnail"),
							Code:  `{% categoryDetail with name="Thumb" %}`,
						},
						{
							Title: w.Tr("CategoryBannerImage"),
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
					Title: w.Tr("SinglePageListTag"),
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
					Title: w.Tr("SinglePageDetailsTag"),
					Link:  "https://www.anqicms.com/manual-category/84.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("SinglePageId"),
							Code:  `{% pageDetail with name="Id" %}`,
						},
						{
							Title: w.Tr("SinglePageTitle"),
							Code:  `{% pageDetail with name="Title" %}`,
						},
						{
							Title: w.Tr("SinglePageLink"),
							Code:  `{% pageDetail with name="Link" %}`,
						},
						{
							Title: w.Tr("SinglePageDescription"),
							Code:  `{% pageDetail with name="Description" %}`,
						},
						{
							Title: w.Tr("SinglePageContent"),
							Code:  `{% pageDetail with name="Content" %}`,
						},
						{
							Title: w.Tr("SinglePageThumbnailImage"),
							Code:  `{% pageDetail with name="Logo" %}`,
						},
						{
							Title: w.Tr("SinglePageThumbnail"),
							Code:  `{% pageDetail with name="Thumb" %}`,
						},
						{
							Title: w.Tr("SinglePageBannerImage"),
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
					Title: w.Tr("DocumentModelDetailsTag"),
					Link:  "https://www.anqicms.com/manual-normal/3489.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("ModelId"),
							Code:  `{% moduleDetail with name="Id" %}`,
						},
						{
							Title: w.Tr("ModelTitle"),
							Code:  `{% moduleDetail with name="Title" %}`,
						},
						{
							Title: w.Tr("ModelLink"),
							Code:  `{% moduleDetail with name="Link" %}`,
						},
						{
							Title: w.Tr("ModelTableName"),
							Code:  `{% moduleDetail with name="TableName" %}`,
						},
					},
				},
			},
		},
		{
			Title: w.Tr("DocumentTag"),
			Type:  "tag",
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("DocumentListTag"),
					Link:  "https://www.anqicms.com/manual-archive/79.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("DocumentListDisplay"),
							Options: []response.DocOption{
								{
									Title: w.Tr("DocumentId"),
									Code:  `{{item.Id}}`,
								},
								{
									Title: w.Tr("DocumentTitle"),
									Code:  `{{item.Title}}`,
								},
								{
									Title: w.Tr("DocumentKeywords"),
									Code:  `{{item.Keywords}}`,
								},
								{
									Title: w.Tr("DocumentDescription"),
									Code:  `{{item.Description}}`,
								},
								{
									Title: w.Tr("DocumentLink"),
									Code:  `{{item.Link}}`,
								},
								{
									Title: w.Tr("Views"),
									Code:  `{{item.Views}}`,
								},
								{
									Title: w.Tr("ReleaseDate"),
									Code:  `{{stampToDate(item.CreatedTime, "2006-01-02")}}`,
								},
								{
									Title: w.Tr("UpdateDate"),
									Code:  `{{stampToDate(item.UpdatedTime, "2006-01-02")}}`,
								},
								{
									Title: w.Tr("CoverFirstImage"),
									Code:  `<img src="{{item.Logo}}" />`,
								},
								{
									Title: w.Tr("Thumbnail"),
									Code:  `<img src="{{item.Thumb}}" />`,
								},
								{
									Title: w.Tr("CoverSetImage"),
									Code: `<ul>
								{% for inner in item.Images %}
								<li>
									<img src="{{inner}}" alt="{{item.Title}}" />
								</li>
								{% endfor %}
								</ul>`,
								},
								{
									Title: w.Tr("DocumentTag"),
									Code: `{% tagList tags with itemId=item.Id limit="10" %}
								{% for item in tags %}
								<a href="{{item.Link}}">{{item.Title}}</a>
								{% endfor %}
								{% endtagList %}`,
								},
								{
									Title: w.Tr("CustomFields"),
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
							Title: w.Tr("PagedDocumentDisplay"),
							Options: []response.DocOption{
								{
									Title: w.Tr("DocumentId"),
									Code:  `{{item.Id}}`,
								},
								{
									Title: w.Tr("DocumentTitle"),
									Code:  `{{item.Title}}`,
								},
								{
									Title: w.Tr("DocumentKeywords"),
									Code:  `{{item.Keywords}}`,
								},
								{
									Title: w.Tr("DocumentDescription"),
									Code:  `{{item.Description}}`,
								},
								{
									Title: w.Tr("DocumentLink"),
									Code:  `{{item.Link}}`,
								},
								{
									Title: w.Tr("Views"),
									Code:  `{{item.Views}}`,
								},
								{
									Title: w.Tr("ReleaseDate"),
									Code:  `{{stampToDate(item.CreatedTime, "2006-01-02")}}`,
								},
								{
									Title: w.Tr("UpdateDate"),
									Code:  `{{stampToDate(item.UpdatedTime, "2006-01-02")}}`,
								},
								{
									Title: w.Tr("CoverFirstImage"),
									Code:  `<img src="{{item.Logo}}" />`,
								},
								{
									Title: w.Tr("Thumbnail"),
									Code:  `<img src="{{item.Thumb}}" />`,
								},
								{
									Title: w.Tr("CoverSetImage"),
									Code: `<ul>
								{% for inner in item.Images %}
								<li>
									<img src="{{inner}}" alt="{{item.Title}}" />
								</li>
								{% endfor %}
								</ul>`,
								},
								{
									Title: w.Tr("DocumentTag"),
									Code: `{% tagList tags with itemId=item.Id limit="10" %}
								{% for item in tags %}
								<a href="{{item.Link}}">{{item.Title}}</a>
								{% endfor %}
								{% endtagList %}`,
								},
								{
									Title: w.Tr("CustomFields"),
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
					Title: w.Tr("DocumentDetailsTag"),
					Link:  "https://www.anqicms.com/manual-archive/80.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("DocumentId"),
							Code:  `{% archiveDetail with name="Id" %}`,
						},
						{
							Title: w.Tr("DocumentTitle"),
							Code:  `{% archiveDetail with name="Title" %}`,
						},
						{
							Title: w.Tr("DocumentKeywords"),
							Code:  `{% archiveDetail with name="Keywords" %}`,
						},
						{
							Title: w.Tr("DocumentLink"),
							Code:  `{% archiveDetail with name="Link" %}`,
						},
						{
							Title: w.Tr("DocumentDescription"),
							Code:  `{% archiveDetail with name="Description" %}`,
						},
						{
							Title: w.Tr("DocumentContent"),
							Code:  `{% archiveDetail archiveContent with name="Content" %}{{archiveContent|safe}}`,
						},
						{
							Title: w.Tr("Views"),
							Code:  `{% archiveDetail with name="Views" %}`,
						},
						{
							Title: w.Tr("ReleaseDate"),
							Code:  `{% archiveDetail with name="CreatedTime" format="2006-01-02 15:04" %}`,
						},
						{
							Title: w.Tr("UpdateDate"),
							Code:  `{% archiveDetail with name="UpdatedTime" format="2006-01-02 15:04" %}`,
						},
						{
							Title: w.Tr("CoverFirstImage"),
							Code:  `<img src="{% archiveDetail with name="Logo" %}" alt=""/>`,
						},
						{
							Title: w.Tr("Thumbnail"),
							Code:  `<img src="{% archiveDetail with name="Thumb" %}" alt=""/>`,
						},
						{
							Title: w.Tr("CoverSetImage"),
							Code: `{% archiveDetail archiveImages with name="Images" %}
						{% for item in archiveImages %}
							<img src="{{item}}" alt=""/>
						{% endfor %}`,
						},
						{
							Title: w.Tr("DocumentClassification"),
							Code:  `<a href="{% categoryDetail with name="Link" %}">{% categoryDetail with name="Title" %}</a>`,
						},
						{
							Title: w.Tr("DocumentTag"),
							Code: `{% tagList tags with limit="10" %}
								{% for item in tags %}
								<a href="{{item.Link}}">{{item.Title}}</a>
								{% endfor %}
								{% endtagList %}`,
						},
						{
							Title: w.Tr("CustomFields"),
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
						{
							Title: w.Tr("ContentTitles"),
							Code: `{% archiveDetail contentTitles with name="ContentTitles" %}
								<div>
								{% for item in contentTitles %}
									<div class="{{item.Tag}}" level="{{item.Level}}">{{item.Prefix}} {{item.Title}}</div>
								{% endfor %}
								</div>`,
						},
					},
				},
				{
					Title: w.Tr("PreviousDocumentTag"),
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
					Title: w.Tr("NextDocumentTag"),
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
					Title: w.Tr("RelatedDocumentsTag"),
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
					Title: w.Tr("DocumentParameterTag"),
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
					Title: w.Tr("DocumentParameterFilterTag"),
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
			Title: w.Tr("DocumentTagsTag"),
			Type:  "tag",
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("DocumentTagListTag"),
					Link:  "https://www.anqicms.com/manual-tag/81.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("NormalTagList"),
							Code: `{% tagList tags with limit="10" %}
						{% for item in tags %}
						<a href="{{item.Link}}">{{item.Title}}</a>
						{% endfor %}
						{% endtagList %}`,
						},
						{
							Title: w.Tr("PagedTagList"),
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
					Title: w.Tr("TagDocumentListTag"),
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
					Title: w.Tr("TagDetailsTag"),
					Link:  "https://www.anqicms.com/manual-tag/90.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("TagId"),
							Code:  `{% tagDetail with name="Id" %}`,
						},
						{
							Title: w.Tr("TagTitle"),
							Code:  `{% tagDetail with name="Title" %}`,
						},
						{
							Title: w.Tr("TagLink"),
							Code:  `{% tagDetail with name="Link" %}`,
						},
						{
							Title: w.Tr("TagDescription"),
							Code:  `{% tagDetail with name="Description" %}`,
						},
						{
							Title: w.Tr("TagIndexLetter"),
							Code:  `{% tagDetail with name="FirstLetter" %}`,
						},
					},
				},
			},
		},
		{
			Title: w.Tr("OtherTags"),
			Type:  "tag",
			Docs: []response.DesignDoc{
				{
					Title: w.Tr("CommentTag"),
					Link:  "https://www.anqicms.com/manual-other/85.html",
					Docs: []response.DesignDoc{
						{
							Title: w.Tr("RegularCommentList"),
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
							Title: w.Tr("PagedCommentList"),
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
							Title: w.Tr("CommentFormSubmission"),
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
					Title: w.Tr("MessageTag"),
					Link:  "https://www.anqicms.com/manual-other/86.html",
					Docs: []response.DesignDoc{
						{
							Title:   w.Tr("DefaultMessageForm"),
							Content: w.Tr("TheFollowingCodeCanBeUsedToLoopAndOutputAllTheSetFields"),
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
							Title:   w.Tr("CustomMessageFormFields"),
							Content: w.Tr("IfYouWantToCustomizeTheFormDisplayYouCanAlsoUseRegularInputToOrganizeTheDisplay"),
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
					Title: w.Tr("PaginationTag"),
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
					Title: w.Tr("FriendlyLinkTag"),
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
					Title: w.Tr("MessageVerificationCodeUseTag"),
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
			Title: w.Tr("FilterFilter"),
			Type:  "filter",
			Docs: []response.DesignDoc{
				{
					Title:   w.Tr("JudgeWhetherItContainsACertainKeyword"),
					Link:    "https://www.anqicms.com/manual-filter/250.html",
					Content: w.Tr("ContainTheFilterCanDetermineWhetherAKeywordIsContainedInALineOfStringArrayMapStructAndTheResultWillReturnABooleanValue"),
					Code:    `{{obj|contain:"关键词"}}`,
				},
				{
					Title:   w.Tr("DeleteLeadingAndTrailingSpacesKeywords"),
					Link:    "https://www.anqicms.com/manual-filter/251.html",
					Content: w.Tr(`trim、trimLeft、trimRight 过滤器可以分别删除字符串首尾空格、特定字符。`),
					Code:    `{{obj|trim}}`,
				},
				{
					Title:   w.Tr("CountTheNumberOfTimesAKeywordAppears"),
					Link:    "https://www.anqicms.com/manual-filter/252.html",
					Content: w.Tr(`count 过滤器可以计算某个关键词在一行字符串或数组（array/slice）中出现的次数。`),
					Code:    `{{obj|count:"关键词"}}`,
				},
				{
					Title:   w.Tr("GetThePositionOfAKeyword"),
					Link:    "https://www.anqicms.com/manual-filter/254.html",
					Content: w.Tr(`index 过滤器可以计算某个关键词在一行字符串或数组（array/slice）中出现的位置。如果字符串中包含多个需要查找的关键词，则index返回的是首次出现的位置。如果没有找到，则返回-1。注意：如果字符串中有中文，则计算位置的时候，一个中文算3个位置。`),
					Code:    `{{obj|index:"关键词"}}`,
				},
				{
					Title:   w.Tr("ReplaceKeywords"),
					Link:    "https://www.anqicms.com/manual-filter/256.html",
					Content: w.Tr(`replace 过滤器可以将字符串中的旧的词old替换词新的词new，返回替换后的新字符串。如果 old 为空，它将在字符串的开头和每个 UTF-8 序列之后进行匹配。如果new为空，则移除old。`),
					Code:    `{{obj|replace:"old,new"}}`,
				},
				{
					Title:   w.Tr("RepeatOutputMultipleTimes"),
					Link:    "https://www.anqicms.com/manual-filter/257.html",
					Content: w.Tr(`repeat 过滤器可以将一个字符串按指定次数重复。`),
					Code:    `{{obj|repeat:次数}}`,
				},
				{
					Title:   w.Tr("AddNumbersStrings"),
					Link:    "https://www.anqicms.com/manual-filter/259.html",
					Content: w.Tr(`add 过滤器可以将怎么将两个数字、字符串相加。add 可以将整数、浮点数、字符串混合相加。也就是在你进行相加计算的时候，可以忽略他们的类型，在自动转换失败的时候，则会忽略相加的内容。`),
					Code:    `{{ obj|add:obj2 }}`,
				},
				{
					Title:   w.Tr("ConvertTheFirstLetterToUppercase"),
					Link:    "https://www.anqicms.com/manual-filter/261.html",
					Content: w.Tr(`capfirst 过滤器可以将英文字符串第一个字母转换为大写。只有英文字母会被转换。`),
					Code:    `{{ obj|capfirst }}`,
				},
				{
					Title:   w.Tr("ConvertEnglishToUppercase"),
					Link:    "https://www.anqicms.com/manual-filter/261.html",
					Content: w.Tr(`upper 过滤器可以将英文字符串中所有的字母转换成大写。`),
					Code:    `{{ obj|upper }}`,
				},
				{
					Title:   w.Tr("ConvertEnglishToLowercase"),
					Link:    "https://www.anqicms.com/manual-filter/261.html",
					Content: w.Tr(`lower 过滤器可以将英文字符串中所有的字母转换成小写。`),
					Code:    `{{ obj|lower }}`,
				},
				{
					Title:   w.Tr("ConvertTheFirstLetterOfEachWordToUppercase"),
					Link:    "https://www.anqicms.com/manual-filter/261.html",
					Content: w.Tr(`title 过滤器可以将英文字符串中所有的单词的首字母转换成大写。`),
					Code:    `{{ obj|title }}`,
				},
				{
					Title:   w.Tr("SetDefaultValue"),
					Link:    "https://www.anqicms.com/manual-filter/265.html",
					Content: w.Tr(`default 过滤器可以在如果数字或字符串、对象没有值的时候给数字或字符串设置默认值。default_if_none 过滤器可以判断指针类型的对象是否为空，如果为空，则设置默认值。`),
					Code:    `{{ obj|default:"默认值" }}`,
				},
				{
					Title:   w.Tr("HtmlCodeIsNotEscaped"),
					Link:    "https://www.anqicms.com/manual-filter/280.html",
					Content: w.Tr(`safe 过滤器可以取消模板输出的默认转义属性，让直接输出html代码到界面，让浏览器解析HTML代码。一般用在富文本输出中，如显示文章详情等情况下。注意：使用 safe 过滤器，默认认为你的输出是安全的，它不会对特殊字符进行转义，因此如果代码中包含有xss注入等问题情况，它也会原样输出。请注意防范风险。`),
					Code:    `{{ obj|safe }}`,
				},
				{
					Title:   w.Tr("TheFirstValueOfAStringOrArray"),
					Link:    "https://www.anqicms.com/manual-filter/269.html",
					Content: w.Tr(`first 过滤器可以获得字符串第一个字符或数组第一个值。如果原字符串、数组为空，什么也不返回。如果字符串是中文，则返回第一个汉字。`),
					Code:    `{{ obj|first }}`,
				},
				{
					Title:   w.Tr("TheLastValueOfAStringOrArray"),
					Link:    "https://www.anqicms.com/manual-filter/269.html",
					Content: w.Tr(`last 过滤器可以获取字符串最后一个字符或数组最后一个值。如果原字符串、数组为空，什么也不返回。如果字符串是中文，则返回最后一个汉字。`),
					Code:    `{{ obj|last }}`,
				},
				{
					Title:   w.Tr("RetainASpecifiedNumberOfDecimalPlaces"),
					Link:    "https://www.anqicms.com/manual-filter/270.html",
					Content: w.Tr(`floatformat 过滤器可以将一个浮点数保留2位小数输出。也可以保留指定的其他位数小数点。如保留小数点后3位等。同时支持负数位数，如果设置的是负数，则从最后一位往前推算。`),
					Code:    `{{ obj|floatformat:2 }}`,
				},
				{
					Title:   w.Tr("GetTheLengthOfAnObject"),
					Link:    "https://www.anqicms.com/manual-filter/274.html",
					Content: w.Tr(`length 过滤器可以获取字符串、数组、键值对的长度。对于字符串，则计算它的utf8实际字符的数量，一个字母为一个，一个汉字也为1个。数组和键值对则计算它们的索引数量。`),
					Code:    `{{ obj|length }}`,
				},
				{
					Title:   w.Tr("ConvertMultipleLinesOfTextToHtml"),
					Link:    "https://www.anqicms.com/manual-filter/275.html",
					Content: w.Tr(`linebreaks 过滤器可以将多行文本按换行符转换成html标签。每行开头和结尾采用<p>和</p>包裹，中间有空行则采用 <br/>。还可以使用 linebreaksbr 来进行处理。与 linebreaks不同的地方是，linebreaksbr只是直接将换行符替换成 <br/>，并且不在开头和结尾添加p标签。还可以使用 linenumbers 来给多行文本的每一行进行标号，符号从1开始。如 1.。`),
					Code:    `{{ obj|linebreaks }}`,
				},
				{
					Title:   w.Tr("RemoveHtmlCode"),
					Link:    "https://www.anqicms.com/manual-filter/279.html",
					Content: w.Tr(`striptags 过滤器可以移除html代码中的所有html标签。removetags 过滤器可以将移除html代码中指定标签。`),
					Code:    `{{ obj|striptags }}`,
				},
				{
					Title:   w.Tr("FormatAnyValueAsAStringOutput"),
					Link:    "https://www.anqicms.com/manual-filter/283.html",
					Content: w.Tr(`stringformat 过滤器可以将数字、字符串、数组等任意值按指定格式格式化成字符串输出。`),
					Code:    `{{ obj|stringformat:"#+v" }}`,
				},
				{
					Title:   w.Tr("InterceptAndAddAString..."),
					Link:    "https://www.anqicms.com/manual-filter/284.html",
					Content: w.Tr(`truncatechars 过滤器可以对字符串进行截取并添加...，该方法会截断单词，指定长度包括...。truncatewords 过滤器可以对字符串按单词进行截取并添加...。`),
					Code:    `{{ obj|truncatechars:50 }}`,
				},
				{
					Title:   w.Tr("HtmlInterceptionAndAddition..."),
					Link:    "https://www.anqicms.com/manual-filter/284.html",
					Content: w.Tr(`truncatechars_html 过滤器可以对html代码进行截取并添加...，该方法会截断单词，指定长度包括...。truncatewords_html 过滤器可以对html代码按单词进行截取并添加...。`),
					Code:    `{{ obj|truncatechars_html:200 }}`,
				},
			},
		},
	}

	return designTplHelpers
}
