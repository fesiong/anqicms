package tags

import (
	"fmt"
	"reflect"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagUserDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagUserDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)

	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	fieldName := ""
	inputName := ""
	if args["name"] != nil {
		inputName = args["name"].String()
		fieldName = library.Case2Camel(inputName)
	}
	// 规定某些字段不能返回内容
	if fieldName == "Password" {
		return nil
	}

	userDetail, ok := ctx.Public["user"].(*model.User)
	if !ok && id == 0 {
		return nil
	}
	//不是同一个，重新获取
	if userDetail != nil && (id > 0 && userDetail.Id != id) {
		userDetail = nil
	}

	if userDetail == nil && id > 0 {
		cacheKey := fmt.Sprintf("userDetail_%d", id)
		if cached, ok := ctx.Private[cacheKey].(*model.User); ok {
			userDetail = cached
		} else {
			userDetail, _ = currentSite.GetUserInfoById(id)
			if userDetail != nil {
				ctx.Private[cacheKey] = userDetail
			}
		}
	}
	if userDetail == nil {
		return nil
	}

	if len(node.name) > 0 && len(fieldName) == 0 {
		ctx.Private[node.name] = userDetail
		return nil
	}

	var content interface{}
	switch fieldName {
	case "Id":
		content = userDetail.Id
	case "CreatedTime":
		content = userDetail.CreatedTime
	case "UpdatedTime":
		content = userDetail.UpdatedTime
	case "ParentId":
		content = userDetail.ParentId
	case "UserName":
		content = userDetail.UserName
	case "UrlToken":
		content = userDetail.UrlToken
	case "RealName":
		content = userDetail.RealName
	case "FirstName":
		content = userDetail.FirstName
	case "LastName":
		content = userDetail.LastName
	case "Birthday":
		content = userDetail.Birthday
	case "AvatarURL":
		content = userDetail.AvatarURL
	case "Introduce":
		content = userDetail.Introduce
	case "Email":
		content = userDetail.Email
	case "Phone":
		content = userDetail.Phone
	case "GroupId":
		content = userDetail.GroupId
	case "Status":
		content = userDetail.Status
	case "Balance":
		content = userDetail.Balance
	case "IsRetailer":
		content = userDetail.IsRetailer
	case "InviteCode":
		content = userDetail.InviteCode
	case "ExpireTime":
		content = userDetail.ExpireTime
	case "Link":
		userDetail.Link = currentSite.GetUrl(provider.PatternPeople, userDetail, 0)
		content = userDetail.Link
	default:
		v := reflect.ValueOf(*userDetail)
		f := v.FieldByName(fieldName)
		if f.IsValid() {
			content = f.Interface()
		} else if extra, ok := userDetail.Extra[inputName]; ok {
			content = extra.Value
		}
	}

	if node.name == "" {
		writer.WriteString(fmt.Sprint(content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagUserDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagUserDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("userDetail-tag needs a user field name.", nil)
	}

	if nameToken.Val == "with" {
		//with 需要退回
		arguments.ConsumeN(-1)
	} else {
		tagNode.name = nameToken.Val
	}

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed userDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
