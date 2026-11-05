package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/flosch/pongo2/v6"

	"kandaoni.com/anqicms/model"
)

// 通知模板渲染器（R72）。安全边界分三层，任何一层都不回显模板内容：
//  1. 词法白名单：渲染前先扫标签与过滤器，出现白名单外的一律拒绝；
//  2. 独立 TemplateSet：每个模板新建一个无 loader 的 set，并屏蔽危险内置标签，
//     即使词法检查漏网也无法读文件或触发项目自定义标签；
//  3. 上下文只放原始类型，访客可控值先净化，群聊不注入私聊字段。
var (
	errNotifyTemplateTagNotAllowed      = errors.New("通知模板包含不允许的标签")
	errNotifyTemplateFilterNotAllowed   = errors.New("通知模板包含不允许的过滤器")
	errNotifyTemplateInvalid            = errors.New("通知模板格式不正确")
	errNotifyTemplateOverrideInvalid    = errors.New("通知模板平台覆盖配置不正确")
	errNotifyTemplatePayloadInvalid     = errors.New("通知事件载荷格式不正确")
	notifyTemplateTagPattern            = regexp.MustCompile(`\{%-?\s*([A-Za-z_][A-Za-z0-9_]*)`)
	notifyTemplateFilterPattern         = regexp.MustCompile(`\|\s*([A-Za-z_][A-Za-z0-9_]*)`)
	notifyTemplateContentMaxRunes       = 1500
	notifyTemplateSubjectMaxRunes       = 255
	notifyTemplateGuestbookContentRunes = 2000
	notifyTemplateFieldMaxRunes         = 200
)

var notifyTemplateAllowedTags = map[string]bool{
	"comment": true, "if": true, "for": true, "with": true, "set": true,
	"firstof": true, "filter": true, "spaceless": true, "autoescape": true,
	"ifchanged": true, "ifequal": true, "ifnotequal": true, "now": true,
	"cycle": true, "templatetag": true, "widthratio": true,
	// 分支与结束标记不是独立标签，但会出现在 {% %} 里，必须放行。
	"else": true, "elif": true, "empty": true,
}

var notifyTemplateAllowedFilters = map[string]bool{
	"default": true, "upper": true, "lower": true, "title": true,
	"capfirst": true, "length": true, "truncatechars": true, "truncatewords": true,
	"date": true, "time": true, "join": true, "cut": true, "add": true,
	"stringformat": true, "yesno": true, "pluralize": true, "center": true,
	"ljust": true, "rjust": true, "wordcount": true, "first": true, "last": true,
}

// notifyBannedPongoTags 是非白名单内置标签的兜底屏蔽。BanTag 对未注册到本 set 的
// 项目自定义标签（system 等）会报「not found」，忽略即可——词法白名单已经拦下它们。
var notifyBannedPongoTags = []string{
	"system", "include", "extends", "import", "ssi", "macro", "lorem", "block",
}

// RenderNotificationText 渲染一条通知模板。
//
// platform 决定平台覆盖；targetKind 决定上下文（群聊只有公共键，私聊才注入
// guestbook/order/agent）。所有错误都是静态文案，绝不回显模板或数据内容。
func RenderNotificationText(tpl *model.NotificationTemplate, platform, eventType, payloadJSON, targetKind, siteName, siteURL string) (subject, text string, err error) {
	if tpl == nil {
		tpl = DefaultNotificationTemplate(eventType)
	}
	if tpl == nil {
		return "", "", errNotifyTemplateInvalid
	}
	subjectTemplate := tpl.Subject
	contentTemplate := tpl.Content

	if strings.TrimSpace(tpl.PlatformOverridesJson) != "" {
		var overrides map[string]struct {
			Subject string `json:"subject"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(tpl.PlatformOverridesJson), &overrides); err != nil {
			return "", "", errNotifyTemplateOverrideInvalid
		}
		if override, exists := overrides[platform]; exists {
			if strings.TrimSpace(override.Subject) != "" {
				subjectTemplate = override.Subject
			}
			if strings.TrimSpace(override.Content) != "" {
				contentTemplate = override.Content
			}
		}
	}

	if err := validateNotificationTemplateSyntax(subjectTemplate); err != nil {
		return "", "", err
	}
	if err := validateNotificationTemplateSyntax(contentTemplate); err != nil {
		return "", "", err
	}

	ctx, err := notificationTemplateContext(eventType, payloadJSON, targetKind, siteName, siteURL)
	if err != nil {
		return "", "", err
	}

	subject, err = renderNotificationTemplateText(subjectTemplate, ctx)
	if err != nil {
		return "", "", errNotifyTemplateInvalid
	}
	text, err = renderNotificationTemplateText(contentTemplate, ctx)
	if err != nil {
		return "", "", errNotifyTemplateInvalid
	}
	return subject, text, nil
}

// validateNotificationTemplateSyntax 是词法白名单主闸门，必须先于渲染执行。
// 允许 end<tag> 形式的结束标记（endif/endfor/endwith/...）。
func validateNotificationTemplateSyntax(template string) error {
	for _, match := range notifyTemplateTagPattern.FindAllStringSubmatch(template, -1) {
		name := match[1]
		if notifyTemplateAllowedTags[name] {
			continue
		}
		if strings.HasPrefix(name, "end") && notifyTemplateAllowedTags[strings.TrimPrefix(name, "end")] {
			continue
		}
		return errNotifyTemplateTagNotAllowed
	}
	for _, match := range notifyTemplateFilterPattern.FindAllStringSubmatch(template, -1) {
		if !notifyTemplateAllowedFilters[match[1]] {
			return errNotifyTemplateFilterNotAllowed
		}
	}
	return nil
}

// renderNotificationTemplateText 用独立的 TemplateSet 渲染单个模板。
// 无 loader：即使漏网也无法读文件；BanTag 是词法白名单之外的第二道闸门。
func renderNotificationTemplateText(template string, ctx pongo2.Context) (string, error) {
	if template == "" {
		return "", nil
	}
	// nil loader 是刻意的：没有 loader 就没有文件系统入口，词法白名单漏掉的
	// include/extends/import 也读不到文件。反过来说，别给这个 set 换成带 loader 的实现，
	// 也别把带 loader 的标签（如项目自定义的 system）放进白名单——那会让模板能读任意文件。
	set := pongo2.NewSet("notify", nil)
	for _, tag := range notifyBannedPongoTags {
		_ = set.BanTag(tag)
	}
	parsed, err := set.FromString(template)
	if err != nil {
		return "", err
	}
	return parsed.Execute(ctx)
}

// notificationTemplateContext 只构造原始类型上下文。
// 公共键对所有目标可见；私聊键仅在 targetKind == private 时注入。
func notificationTemplateContext(eventType, payloadJSON, targetKind, siteName, siteURL string) (pongo2.Context, error) {
	var payload map[string]any
	if strings.TrimSpace(payloadJSON) != "" {
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			return nil, errNotifyTemplatePayloadInvalid
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}

	occurred := notificationPayloadTime(eventType, payload)
	// occurred_time 是唯一的非原始类型：pongo2 的 date/time 过滤器要求 time.Time，
	// 传字符串会在过滤器里报类型错误。occurred_at 是它的预格式化副本（"2006-01-02 15:04:05"），
	// 供不需要 date 过滤器的模板直接用；其余键一律 string，不放任何模型对象。
	ctx := pongo2.Context{
		"site_name":     sanitizeNotificationValue(siteName, notifyTemplateFieldMaxRunes),
		"site_url":      sanitizeNotificationValue(siteURL, notifyTemplateFieldMaxRunes),
		"event_type":    sanitizeNotificationValue(eventType, notifyTemplateFieldMaxRunes),
		"event_label":   notificationEventLabel(eventType),
		"business_id":   sanitizeNotificationValue(notificationBusinessId(payload), notifyTemplateFieldMaxRunes),
		"occurred_at":   occurred.Format("2006-01-02 15:04:05"),
		"occurred_time": occurred,
		"amount":        notificationAmountYuan(payload["amount"]),
		"status":        sanitizeNotificationValue(notificationString(payload["status"]), notifyTemplateFieldMaxRunes),
		"order_type":    sanitizeNotificationValue(notificationString(payload["order_type"]), notifyTemplateFieldMaxRunes),
	}

	if targetKind != TargetKindPrivate {
		return ctx, nil
	}
	ctx["guestbook"] = map[string]any{
		"user_name": sanitizeNotificationValue(notificationString(payload["user_name"]), notifyTemplateFieldMaxRunes),
		"contact":   sanitizeNotificationValue(notificationString(payload["contact"]), notifyTemplateFieldMaxRunes),
		"content":   sanitizeNotificationValue(notificationString(payload["content"]), notifyTemplateGuestbookContentRunes),
	}
	ctx["order"] = map[string]any{
		"buyer_name":    sanitizeNotificationValue(notificationString(payload["buyer_name"]), notifyTemplateFieldMaxRunes),
		"buyer_phone":   sanitizeNotificationValue(notificationString(payload["buyer_phone"]), notifyTemplateFieldMaxRunes),
		"buyer_email":   sanitizeNotificationValue(notificationString(payload["buyer_email"]), notifyTemplateFieldMaxRunes),
		"buyer_address": sanitizeNotificationValue(notificationString(payload["buyer_address"]), notifyTemplateFieldMaxRunes),
	}
	ctx["agent"] = map[string]any{
		"action": sanitizeNotificationValue(notificationString(payload["action"]), notifyTemplateFieldMaxRunes),
		"error":  sanitizeNotificationValue(notificationString(payload["error"]), notifyTemplateFieldMaxRunes),
	}
	return ctx, nil
}

// sanitizeNotificationValue 去掉控制字符、把尖括号换成全角，再截断到 maxRunes。
// 目的是防止 <at user_id="..."> 之类的平台提及/卡片标记从访客内容注入。
func sanitizeNotificationValue(value string, maxRunes int) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			builder.WriteRune(' ')
		case unicode.IsControl(r):
			// 丢弃其余控制字符。
		case r == '<':
			builder.WriteRune('＜')
		case r == '>':
			builder.WriteRune('＞')
		default:
			builder.WriteRune(r)
		}
	}
	cleaned := strings.Join(strings.Fields(builder.String()), " ")
	if maxRunes > 0 {
		if runes := []rune(cleaned); len(runes) > maxRunes {
			cleaned = string(runes[:maxRunes])
		}
	}
	return cleaned
}

// truncateNotificationContent 把最终文本压到 1500 个 rune。
// 适配器分段发送不是原子的，重试会产生重复段/半截消息，所以只发一段。
func truncateNotificationContent(content string) string {
	runes := []rune(content)
	if len(runes) <= notifyTemplateContentMaxRunes {
		return content
	}
	return string(runes[:notifyTemplateContentMaxRunes]) + "\n…（内容已截断）"
}

// truncateNotificationSubject 按 varchar(255) 截断标题，防止渲染后的超长标题写库失败。
func truncateNotificationSubject(subject string) string {
	runes := []rune(subject)
	if len(runes) <= notifyTemplateSubjectMaxRunes {
		return subject
	}
	return string(runes[:notifyTemplateSubjectMaxRunes])
}

func notificationEventLabel(eventType string) string {
	switch eventType {
	case NotifyEventGuestbookCreated:
		return "新留言"
	case NotifyEventOrderCreated:
		return "新订单"
	case NotifyEventOrderPaid:
		return "订单支付成功"
	case NotifyEventAgentFailed:
		return "智能体任务失败"
	default:
		return eventType
	}
}

// notificationString 把 JSON 解出来的任意值转成字符串。
// 数字统一走整型（避免 float64 打出 123.000000 这类形态）。
func notificationString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	default:
		return fmt.Sprint(typed)
	}
}

func notificationBusinessId(payload map[string]any) string {
	for _, key := range []string{"order_id", "guestbook_id", "audit_id"} {
		if value, exists := payload[key]; exists {
			if text := notificationString(value); text != "" {
				return text
			}
		}
	}
	return ""
}

// notificationPayloadTime 取业务发生时间。
// order.paid 的语义时间是「支付时间」：订单可能创建很久后才支付，用 created_time 会把
// 群里显示的时间写成下单时间，所以该事件优先 paid_time；其余事件保持 created_time 优先。
// 两个键都缺失时回落当前时间。
func notificationPayloadTime(eventType string, payload map[string]any) time.Time {
	keys := []string{"created_time", "paid_time"}
	if eventType == NotifyEventOrderPaid {
		keys = []string{"paid_time", "created_time"}
	}
	for _, key := range keys {
		value, exists := payload[key]
		if !exists {
			continue
		}
		var stamp int64
		switch typed := value.(type) {
		case float64:
			stamp = int64(typed)
		case int64:
			stamp = typed
		case int:
			stamp = int64(typed)
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				stamp = parsed
			}
		}
		if stamp > 0 {
			return time.Unix(stamp, 0)
		}
	}
	return time.Now()
}

// notificationAmountYuan 把「分」转成两位小数的「元」字符串。
func notificationAmountYuan(value any) string {
	cents, ok := notificationCents(value)
	if !ok {
		return ""
	}
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

func notificationCents(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// DefaultNotificationTemplate 返回四类事件的默认模板。
// 只使用公共键（群安全）；规则 TemplateId == 0 或模板行不存在时使用。
func DefaultNotificationTemplate(eventType string) *model.NotificationTemplate {
	label := notificationEventLabel(eventType)
	return &model.NotificationTemplate{
		Key:     eventType,
		Name:    label,
		Subject: "[{{ site_name }}] {{ event_label }}",
		Content: "[{{ site_name }}] {{ event_label }}\n编号：{{ business_id }}\n时间：{{ occurred_at }}",
	}
}
