package tags

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

type tagContactNode struct {
	name string
	args map[string]pongo2.IEvaluator
}

func (node *tagContactNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	var content string

	if currentSite.Contact != nil {
		switch fieldName {
		case "UserName":
			content = currentSite.Contact.UserName
		case "Cellphone":
			content = currentSite.Contact.Cellphone
		case "Address":
			content = currentSite.Contact.Address
		case "Email":
			content = currentSite.Contact.Email
		case "Wechat":
			content = currentSite.Contact.Wechat
		case "QQ":
			content = currentSite.Contact.QQ
		case "WhatsApp":
			content = currentSite.Contact.WhatsApp
		case "Facebook":
			content = currentSite.Contact.Facebook
		case "Twitter":
			content = currentSite.Contact.Twitter
		case "Tiktok":
			content = currentSite.Contact.Tiktok
		case "Pinterest":
			content = currentSite.Contact.Pinterest
		case "Linkedin":
			content = currentSite.Contact.Linkedin
		case "Instagram":
			content = currentSite.Contact.Instagram
		case "Youtube":
			content = currentSite.Contact.Youtube
		case "Qrcode":
			content = currentSite.Contact.Qrcode
			if !strings.HasPrefix(content, "http") && !strings.HasPrefix(content, "//") {
				content = currentSite.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(content, "/")
			}
		default:
			if currentSite.Contact.ExtraFields != nil {
				for i := range currentSite.Contact.ExtraFields {
					if currentSite.Contact.ExtraFields[i].Name == fieldName {
						content = fmt.Sprint(currentSite.Contact.ExtraFields[i].Value)
						if content == "" && currentSite.Contact.ExtraFields[i].Content != "" {
							content = currentSite.Contact.ExtraFields[i].Content
						}
						break
					}
				}
			}
			// 备选方案：使用反射获取
			if content == "" {
				v := reflect.ValueOf(*currentSite.Contact)
				f := v.FieldByName(fieldName)
				if f.IsValid() {
					content = fmt.Sprintf("%v", f.Interface())
				}
			}
		}
	}

	// output
	if node.name == "" {
		writer.WriteString(content)
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagContactParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagContactNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("contact-tag needs a accept name.", nil)
	}

	if nameToken.Val == "with" {
		//with 需要退回
		arguments.ConsumeN(-1)
	} else {
		tagNode.name = nameToken.Val
	}

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed contact-tag arguments.", nil)
	}

	return tagNode, nil
}
