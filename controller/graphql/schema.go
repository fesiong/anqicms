package graphql

import (
	"github.com/graphql-go/graphql"
)

// Schema 定义GraphQL Schema
var Schema graphql.Schema

// 初始化Schema
func init() {
	// 定义类型
	metadataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PageMeta",
		Fields: graphql.Fields{
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"image": &graphql.Field{
				Type: graphql.String,
			},
			"nav_bar": &graphql.Field{
				Type: graphql.Int,
			},
			"page_id": &graphql.Field{
				Type: graphql.Int,
			},
			"module_id": &graphql.Field{
				Type: graphql.Int,
			},
			"page_name": &graphql.Field{
				Type: graphql.String,
			},
			"canonical_url": &graphql.Field{
				Type: graphql.String,
			},
			"total_pages": &graphql.Field{
				Type: graphql.Int,
			},
			"current_page": &graphql.Field{
				Type: graphql.Int,
			},
			"status_code": &graphql.Field{
				Type: graphql.Int,
			},
			"params": &graphql.Field{
				Type: JSONScalar,
			},
		},
	})

	categoryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Category",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"seo_title": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.String,
			},
			"url_token": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
			"module_id": &graphql.Field{
				Type: graphql.Int,
			},
			"parent_id": &graphql.Field{
				Type: graphql.Int,
			},
			"type": &graphql.Field{
				Type: graphql.Int,
			},
			"sort": &graphql.Field{
				Type: graphql.Int,
			},
			"template": &graphql.Field{
				Type: graphql.String,
			},
			"detail_template": &graphql.Field{
				Type: graphql.String,
			},
			"is_inherit": &graphql.Field{
				Type: graphql.Int,
			},
			"images": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
			"logo": &graphql.Field{
				Type: graphql.String,
			},
			"extra": &graphql.Field{
				Type: JSONScalar, // 需要确认是否已定义
			},
			"archive_count": &graphql.Field{
				Type: graphql.Int,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"spacer": &graphql.Field{
				Type: graphql.String,
			},
			"has_children": &graphql.Field{
				Type: graphql.Boolean,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"thumb": &graphql.Field{
				Type: graphql.String,
			},
			"is_current": &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	})

	tagType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Tag",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"category_id": &graphql.Field{
				Type: graphql.Int,
			},
			"seo_title": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.String,
			},
			"url_token": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"first_letter": &graphql.Field{
				Type: graphql.String,
			},
			"template": &graphql.Field{
				Type: graphql.String,
			},
			"logo": &graphql.Field{
				Type: graphql.String,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"thumb": &graphql.Field{
				Type: graphql.String,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
			"category_title": &graphql.Field{
				Type: graphql.String,
			},
			"extra": &graphql.Field{
				Type: JSONScalar, // 使用已定义的JSON标量类型
			},
		},
	})

	tagListType := graphql.NewObject(graphql.ObjectConfig{
		Name: "TagList",
		Fields: graphql.Fields{
			"total": &graphql.Field{
				Type: graphql.Int,
			},
			"items": &graphql.Field{
				Type: graphql.NewList(tagType),
			},
			"page": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})

	var archiveType *graphql.Object
	archiveType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Archive",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"parent_id": &graphql.Field{
				Type: graphql.Int,
			},
			"created_time": &graphql.Field{
				Type: graphql.Int,
			},
			"updated_time": &graphql.Field{
				Type: graphql.Int,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"seo_title": &graphql.Field{
				Type: graphql.String,
			},
			"url_token": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"module_id": &graphql.Field{
				Type: graphql.Int,
			},
			"category_id": &graphql.Field{
				Type: graphql.Int,
			},
			"views": &graphql.Field{
				Type: graphql.Int,
			},
			"comment_count": &graphql.Field{
				Type: graphql.Int,
			},
			"images": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
			"template": &graphql.Field{
				Type: graphql.String,
			},
			"canonical_url": &graphql.Field{
				Type: graphql.String,
			},
			"fixed_link": &graphql.Field{
				Type: graphql.String,
			},
			"user_id": &graphql.Field{
				Type: graphql.Int,
			},
			"price": &graphql.Field{
				Type: graphql.Int,
			},
			"origin_price": &graphql.Field{
				Type: graphql.Int,
			},
			"cost_price": &graphql.Field{
				Type: graphql.Int,
			},
			"stock": &graphql.Field{
				Type: graphql.Int,
			},
			"read_level": &graphql.Field{
				Type: graphql.Int,
			},
			"password": &graphql.Field{
				Type: graphql.String,
			},
			"sort": &graphql.Field{
				Type: graphql.Int,
			},
			"has_pseudo": &graphql.Field{
				Type: graphql.Int,
			},
			"keyword_id": &graphql.Field{
				Type: graphql.Int,
			},
			"origin_url": &graphql.Field{
				Type: graphql.String,
			},
			"origin_title": &graphql.Field{
				Type: graphql.String,
			},
			"origin_id": &graphql.Field{
				Type: graphql.Int,
			},
			"video_url": &graphql.Field{
				Type: graphql.String,
			},
			"need_logistics": &graphql.Field{
				Type: graphql.Boolean,
			},
			"is_free_shipping": &graphql.Field{
				Type: graphql.Boolean,
			},
			"sold_count": &graphql.Field{
				Type: graphql.Int,
			},
			"review_count": &graphql.Field{
				Type: graphql.Int,
			},
			"favorite_count": &graphql.Field{
				Type: graphql.Int,
			},
			"option_type": &graphql.Field{
				Type: graphql.Int,
			},
			"weight": &graphql.Field{
				Type: graphql.Float,
			},
			"weight_unit": &graphql.Field{
				Type: graphql.String,
			},
			"is_wholesale": &graphql.Field{
				Type: graphql.Boolean,
			},
			"allow_oversold": &graphql.Field{
				Type: graphql.Boolean,
			},
			"has_order_fields": &graphql.Field{
				Type: graphql.Boolean,
			},
			// 关联对象和计算字段
			"category": &graphql.Field{
				Type: categoryType,
			},
			//"parent": &graphql.Field{
			//	Type: archiveType,
			//},
			"module_name": &graphql.Field{
				Type: graphql.String,
			},
			"logo": &graphql.Field{
				Type: graphql.String,
			},
			"thumb": &graphql.Field{
				Type: graphql.String,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"tags": &graphql.Field{
				Type: graphql.NewList(tagType),
			},
			"has_ordered": &graphql.Field{
				Type: graphql.Boolean,
			},
			"favorable_price": &graphql.Field{
				Type: graphql.Int,
			},
			"has_password": &graphql.Field{
				Type: graphql.Boolean,
			},
			"password_valid": &graphql.Field{
				Type: graphql.Boolean,
			},
			"category_titles": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
			"category_ids": &graphql.Field{
				Type: graphql.NewList(graphql.Int),
			},
			"flag": &graphql.Field{
				Type: graphql.String,
			},
			"type": &graphql.Field{
				Type: graphql.String,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
			"is_favorite": &graphql.Field{
				Type: graphql.Boolean,
			},
			"extra": &graphql.Field{
				Type: JSONScalar, // 需要确认是否已定义
			},
		},
	})

	archiveListType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ArchiveList",
		Fields: graphql.Fields{
			"total": &graphql.Field{
				Type: graphql.Int,
			},
			"items": &graphql.Field{
				Type: graphql.NewList(archiveType),
			},
			"page": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})

	filterGroupType := graphql.NewObject(graphql.ObjectConfig{
		Name: "FilterGroup",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"field_name": &graphql.Field{
				Type: graphql.String,
			},
			"range": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "Range",
					Fields: graphql.Fields{
						"min": &graphql.Field{Type: graphql.Int},
						"max": &graphql.Field{Type: graphql.Int},
					},
				}),
			},
			"items": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "FilterItem",
					Fields: graphql.Fields{
						"label": &graphql.Field{
							Type: graphql.String,
						},
						"link": &graphql.Field{
							Type: graphql.String,
						},
						"is_current": &graphql.Field{
							Type: graphql.Boolean,
						},
						"total": &graphql.Field{
							Type: graphql.Int,
						},
					},
				})),
			},
		},
	})

	groupType := graphql.NewObject(graphql.ObjectConfig{
		Name: "UserGroup",
		Fields: graphql.Fields{
			"id":              &graphql.Field{Type: graphql.Int},
			"title":           &graphql.Field{Type: graphql.String},
			"description":     &graphql.Field{Type: graphql.String},
			"level":           &graphql.Field{Type: graphql.Int},
			"status":          &graphql.Field{Type: graphql.Int},
			"price":           &graphql.Field{Type: graphql.Int},
			"favorable_price": &graphql.Field{Type: graphql.Int},
			"setting": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "UserGroupSetting",
					Fields: graphql.Fields{
						"share_reward":       &graphql.Field{Type: graphql.Int},
						"parent_reward":      &graphql.Field{Type: graphql.Int},
						"discount":           &graphql.Field{Type: graphql.Int},
						"expire_day":         &graphql.Field{Type: graphql.Int},
						"content_no_verify":  &graphql.Field{Type: graphql.Boolean},
						"content_no_captcha": &graphql.Field{Type: graphql.Boolean},
					},
				}),
			},
		},
	})

	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"parent_id": &graphql.Field{
				Type: graphql.Int,
			},
			"user_name": &graphql.Field{
				Type: graphql.String,
			},
			"url_token": &graphql.Field{
				Type: graphql.String,
			},
			"real_name": &graphql.Field{
				Type: graphql.String,
			},
			"avatar_url": &graphql.Field{
				Type: graphql.String,
			},
			"introduce": &graphql.Field{
				Type: graphql.String,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"email_verified": &graphql.Field{
				Type: graphql.Boolean,
			},
			"phone": &graphql.Field{
				Type: graphql.String,
			},
			"group_id": &graphql.Field{
				Type: graphql.Int,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"is_retailer": &graphql.Field{
				Type: graphql.Int,
			},
			"balance": &graphql.Field{
				Type: graphql.Int,
			},
			"total_reward": &graphql.Field{
				Type: graphql.Int,
			},
			"invite_code": &graphql.Field{
				Type: graphql.String,
			},
			"last_login": &graphql.Field{
				Type: graphql.Int,
			},
			"expire_time": &graphql.Field{
				Type: graphql.Int,
			},
			"extra": &graphql.Field{
				Type: JSONScalar, // 自定义字段，使用JSON标量类型
			},
			"token": &graphql.Field{
				Type: graphql.String,
			},
			"full_avatar_url": &graphql.Field{
				Type: graphql.String,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			// 关联对象
			"group": &graphql.Field{
				Type: groupType,
			},
		},
	})

	pageType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Page",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"seo_title": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.String,
			},
			"url_token": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
			"images": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
			"logo": &graphql.Field{
				Type: graphql.String,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"thumb": &graphql.Field{
				Type: graphql.String,
			},
			"is_current": &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	})

	moduleType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Module",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"table_name": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"url_token": &graphql.Field{
				Type: graphql.String,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"is_system": &graphql.Field{
				Type: graphql.Int,
			},
			"title_name": &graphql.Field{
				Type: graphql.String,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	var commentType graphql.Object
	commentType = *graphql.NewObject(graphql.ObjectConfig{
		Name: "Comment",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"archive_id": &graphql.Field{
				Type: graphql.Int,
			},
			"user_id": &graphql.Field{
				Type: graphql.Int,
			},
			"user_name": &graphql.Field{
				Type: graphql.String,
			},
			"ip": &graphql.Field{
				Type: graphql.String,
			},
			"vote_count": &graphql.Field{
				Type: graphql.Int,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
			"parent_id": &graphql.Field{
				Type: graphql.Int,
			},
			"to_uid": &graphql.Field{
				Type: graphql.Int,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"item_title": &graphql.Field{
				Type: graphql.String,
			},
			"parent": &graphql.Field{
				Type: &commentType,
			},
			"active": &graphql.Field{
				Type: graphql.Boolean,
			},
			"created_time": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})

	commentListType := graphql.NewObject(graphql.ObjectConfig{
		Name: "CommentList",
		Fields: graphql.Fields{
			"total": &graphql.Field{
				Type: graphql.Int,
			},
			"items": &graphql.Field{
				Type: graphql.NewList(&commentType),
			},
			"page": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})

	// 需要添加 ExtraField 类型定义
	extraFieldType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ExtraField",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"type": &graphql.Field{
				Type: graphql.String,
			},
			"value": &graphql.Field{
				Type: JSONScalar, // 使用已定义的JSON标量类型
			},
			"remark": &graphql.Field{
				Type: graphql.String,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	customFieldType := graphql.NewObject(graphql.ObjectConfig{
		Name: "CustomField",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"field_name": &graphql.Field{
				Type: graphql.String,
			},
			"type": &graphql.Field{
				Type: graphql.String,
			},
			"required": &graphql.Field{
				Type: graphql.Boolean,
			},
			"is_system": &graphql.Field{
				Type: graphql.Boolean,
			},
			"is_filter": &graphql.Field{
				Type: graphql.Boolean,
			},
			"follow_level": &graphql.Field{
				Type: graphql.Boolean,
			},
			"content": &graphql.Field{
				Type: graphql.String,
			},
			"items": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
		},
	})

	systemSettingType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SystemSetting",
		Fields: graphql.Fields{
			"site_name": &graphql.Field{
				Type: graphql.String,
			},
			"site_logo": &graphql.Field{
				Type: graphql.String,
			},
			"site_icp": &graphql.Field{
				Type: graphql.String,
			},
			"site_copyright": &graphql.Field{
				Type: graphql.String,
			},
			"base_url": &graphql.Field{
				Type: graphql.String,
			},
			"mobile_url": &graphql.Field{
				Type: graphql.String,
			},
			"admin_url": &graphql.Field{
				Type: graphql.String,
			},
			"site_close": &graphql.Field{
				Type: graphql.Int,
			},
			"site_close_tips": &graphql.Field{
				Type: graphql.String,
			},
			"ban_spider": &graphql.Field{
				Type: graphql.Int,
			},
			"template_name": &graphql.Field{
				Type: graphql.String,
			},
			"template_type": &graphql.Field{
				Type: graphql.Int,
			},
			"template_url": &graphql.Field{
				Type: graphql.String,
			},
			"language": &graphql.Field{
				Type: graphql.String,
			},
			"favicon": &graphql.Field{
				Type: graphql.String,
			},
			"default_site": &graphql.Field{
				Type: graphql.Boolean,
			},
			"currency": &graphql.Field{
				Type: graphql.String,
			},
			"extra_fields": &graphql.Field{
				Type: graphql.NewList(extraFieldType),
			},
		},
	})

	contactSettingType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ContactSetting",
		Fields: graphql.Fields{
			"user_name": &graphql.Field{
				Type: graphql.String,
			},
			"cellphone": &graphql.Field{
				Type: graphql.String,
			},
			"address": &graphql.Field{
				Type: graphql.String,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"wechat": &graphql.Field{
				Type: graphql.String,
			},
			"qq": &graphql.Field{
				Type: graphql.String,
			},
			"whats_app": &graphql.Field{
				Type: graphql.String,
			},
			"facebook": &graphql.Field{
				Type: graphql.String,
			},
			"twitter": &graphql.Field{
				Type: graphql.String,
			},
			"tiktok": &graphql.Field{
				Type: graphql.String,
			},
			"pinterest": &graphql.Field{
				Type: graphql.String,
			},
			"linkedin": &graphql.Field{
				Type: graphql.String,
			},
			"instagram": &graphql.Field{
				Type: graphql.String,
			},
			"youtube": &graphql.Field{
				Type: graphql.String,
			},
			"qrcode": &graphql.Field{
				Type: graphql.String,
			},
			"extra_fields": &graphql.Field{
				Type: graphql.NewList(extraFieldType),
			},
		},
	})

	indexSettingType := graphql.NewObject(graphql.ObjectConfig{
		Name: "IndexSetting",
		Fields: graphql.Fields{
			"seo_title": &graphql.Field{
				Type: graphql.String,
			},
			"seo_keywords": &graphql.Field{
				Type: graphql.String,
			},
			"seo_description": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	bannerType := graphql.NewObject(graphql.ObjectConfig{
		Name: "BannerItem",
		Fields: graphql.Fields{
			"logo": &graphql.Field{
				Type: graphql.String,
			},
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"alt": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"type": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	languageType := graphql.NewObject(graphql.ObjectConfig{
		Name: "MultiLangSite",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"status": &graphql.Field{
				Type: graphql.Boolean,
			},
			"parent_id": &graphql.Field{
				Type: graphql.Int,
			},
			"sync_time": &graphql.Field{
				Type: graphql.Int,
			},
			"language_icon": &graphql.Field{
				Type: graphql.String,
			},
			"language_emoji": &graphql.Field{
				Type: graphql.String,
			},
			"language_name": &graphql.Field{
				Type: graphql.String,
			},
			"language": &graphql.Field{
				Type: graphql.String,
			},
			"is_current": &graphql.Field{
				Type: graphql.Boolean,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"base_url": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	friendLinkType := graphql.NewObject(graphql.ObjectConfig{
		Name: "FriendLink",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"back_link": &graphql.Field{
				Type: graphql.String,
			},
			"my_title": &graphql.Field{
				Type: graphql.String,
			},
			"my_link": &graphql.Field{
				Type: graphql.String,
			},
			"contact": &graphql.Field{
				Type: graphql.String,
			},
			"remark": &graphql.Field{
				Type: graphql.String,
			},
			"nofollow": &graphql.Field{
				Type: graphql.Int,
			},
			"sort": &graphql.Field{
				Type: graphql.Int,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"checked_time": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})

	var navType graphql.Object

	navType = *graphql.NewObject(graphql.ObjectConfig{
		Name: "Nav",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"sub_title": &graphql.Field{
				Type: graphql.String,
			},
			"description": &graphql.Field{
				Type: graphql.String,
			},
			"parent_id": &graphql.Field{
				Type: graphql.Int,
			},
			"nav_type": &graphql.Field{
				Type: graphql.Int,
			},
			"page_id": &graphql.Field{
				Type: graphql.Int,
			},
			"type_id": &graphql.Field{
				Type: graphql.Int,
			},
			"link": &graphql.Field{
				Type: graphql.String,
			},
			"logo": &graphql.Field{
				Type: graphql.String,
			},
			"style": &graphql.Field{
				Type: graphql.String,
			},
			"sort": &graphql.Field{
				Type: graphql.Int,
			},
			"status": &graphql.Field{
				Type: graphql.Int,
			},
			"is_current": &graphql.Field{
				Type: graphql.Boolean,
			},
			"spacer": &graphql.Field{
				Type: graphql.String,
			},
			"level": &graphql.Field{
				Type: graphql.Int,
			},
			"thumb": &graphql.Field{
				Type: graphql.String,
			},
			// 对于嵌套的 nav_list，创建递归引用
			"nav_list": &graphql.Field{
				Type: graphql.NewList(&navType),
			},
		},
	})

	// 定义查询
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"metadata": &graphql.Field{
				Type: metadataType,
				Args: graphql.FieldConfigArgument{
					"path": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"params": &graphql.ArgumentConfig{
						Type: JSONScalar,
					},
				},
				Resolve: resolvePageMeta,
			},
			"archive": &graphql.Field{
				Type: archiveType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"filename": &graphql.ArgumentConfig{ // = prev | next 的时候，为特殊查询，查询上下篇
						Type: graphql.String,
					},
					"url_token": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"password": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: resolveArchive,
			},
			"archives": &graphql.Field{
				Type: archiveListType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"parent_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"category_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"category_ids": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.Int),
					},
					"exclude_category_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"exclude_category_ids": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.Int),
					},
					"module_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"author_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"user_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"show_flag": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"show_content": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"show_extra": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"draft": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"child": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"order": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"tag": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"tag_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"flag": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"q": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"like": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"keywords": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"type": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"offset": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"page": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: resolveArchives,
			},
			"filters": &graphql.Field{
				Type: graphql.NewList(filterGroupType),
				Args: graphql.FieldConfigArgument{
					"module_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"show_all": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"all_text": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"show_price": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolveFilters,
			},
			"archiveParams": &graphql.Field{
				Type: graphql.NewList(customFieldType),
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"filename": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"url_token": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolveArchiveParams,
			},
			"user": &graphql.Field{
				Type: userType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: resolveUser,
			},
			"category": &graphql.Field{
				Type: categoryType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"filename": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"catname": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"url_token": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolveCategory,
			},
			"categories": &graphql.Field{
				Type: graphql.NewList(categoryType),
				Args: graphql.FieldConfigArgument{
					"parent_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"module_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"all": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"offset": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: resolveCategories,
			},
			"page": &graphql.Field{
				Type: pageType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"filename": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"url_token": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolvePage,
			},
			"pages": &graphql.Field{
				Type: graphql.NewList(pageType),
				Args: graphql.FieldConfigArgument{
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"offset": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: resolvePages,
			},
			"tag": &graphql.Field{
				Type: tagType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"filename": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"url_token": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolveTag,
			},
			"tags": &graphql.Field{
				Type: tagListType,
				Args: graphql.FieldConfigArgument{
					"item_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"category_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"category_ids": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.Int),
					},
					"type": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"letter": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"order": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"page": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"offset": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: resolveTags,
			},
			"module": &graphql.Field{
				Type: moduleType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"filename": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"url_token": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: resolveModule,
			},
			"modules": &graphql.Field{
				Type:    graphql.NewList(moduleType),
				Resolve: resolveModules,
			},
			"comments": &graphql.Field{
				Type: commentListType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{ // archiveId
						Type: graphql.Int,
					},
					"user_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"order": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"page": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"offset": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolveComments,
			},
			"system": &graphql.Field{
				Type:    systemSettingType,
				Args:    graphql.FieldConfigArgument{},
				Resolve: resolveSystemSetting,
			},
			"contact": &graphql.Field{
				Type:    contactSettingType,
				Args:    graphql.FieldConfigArgument{},
				Resolve: resolveContactSetting,
			},
			"index": &graphql.Field{
				Type:    indexSettingType,
				Args:    graphql.FieldConfigArgument{},
				Resolve: resolveIndexSetting,
			},
			"diy": &graphql.Field{
				Type: graphql.NewList(extraFieldType),
				Args: graphql.FieldConfigArgument{
					"render": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: resolveDiyFields,
			},
			"guestbookFields": &graphql.Field{
				Type:    graphql.NewList(customFieldType),
				Args:    graphql.FieldConfigArgument{},
				Resolve: resolveGuestbookFields,
			},
			"banners": &graphql.Field{
				Type: graphql.NewList(bannerType),
				Args: graphql.FieldConfigArgument{
					"type": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: resolveBanners,
			},
			"languages": &graphql.Field{
				Type:    graphql.NewList(languageType),
				Resolve: resolveLanguages,
			},
			"friendLinks": &graphql.Field{
				Type:    graphql.NewList(friendLinkType),
				Resolve: resolveFriendLinks,
			},
			"navs": &graphql.Field{
				Type: graphql.NewList(&navType),
				Args: graphql.FieldConfigArgument{
					"type_id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"show_type": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: resolveNavs,
			},
		},
	})

	// 创建Schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})

	if err != nil {
		panic(err)
	}

	Schema = schema
}
