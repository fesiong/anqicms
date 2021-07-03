package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
)

type tagGuestbookNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagGuestbookNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	fields := config.GetGuestbookFields()
	for i := range fields {
		//分割items
		fields[i].SplitContent()
	}

	ctx.Private[node.name] = fields
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagGuestbookParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagGuestbookNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("guestbook-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed guestbook-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endguestbook")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endguestbook' must equal to 'guestbook'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endguestbook'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
