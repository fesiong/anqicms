package tags

import (
	"fmt"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagAttachmentNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagAttachmentNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := int64(0)

	var attachment *model.Attachment

	if args["id"] != nil {
		id = int64(args["id"].Integer())
		attachment, _ = currentSite.GetAttachmentById(uint(id))
	}
	if args["name"] != nil {
		name := args["name"].String()
		if after, ok := strings.CutPrefix(name, currentSite.PluginStorage.StorageUrl); ok {
			name = after
		}
		name = strings.TrimPrefix(name, "/")
		attachment, _ = currentSite.GetAttachmentByFileLocation(name)
	}

	ctx.Private[node.name] = attachment

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagAttachmentParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagAttachmentNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("attachment-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed attachment-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endattachment")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endattachment' must equal to 'attachment'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endattachment'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
