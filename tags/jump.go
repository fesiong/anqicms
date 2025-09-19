package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/provider"
	"strconv"
)

// tagJumpNode 翻译
type tagJumpNode struct {
	args []pongo2.IEvaluator
}

func (node *tagJumpNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	var args []interface{}
	for _, value := range node.args {
		val, err := value.Evaluate(ctx)
		if err != nil {
			return err
		}
		args = append(args, val.Interface())
	}

	// args[0] = jump link
	// args[1] = jump type

	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	var jumpLink string
	jumpType := 302
	if len(args) > 0 {
		jumpLink = fmt.Sprintf("%v", args[0])
	}
	if len(args) > 1 {
		val := fmt.Sprintf("%v", args[1])
		jumpType, _ = strconv.Atoi(val)
	}

	currentSite.CtxOri().Redirect(jumpLink, jumpType)
	currentSite.CtxOri().StopExecution()

	return nil
}

func TagJumpParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagJumpNode{
		args: []pongo2.IEvaluator{},
	}

	var args []pongo2.IEvaluator
	for arguments.Remaining() > 0 {
		valueExpr, err := arguments.ParseExpression()
		if err != nil {
			return nil, arguments.Error("Can not parse with args.", nil)
		}
		args = append(args, valueExpr)
	}

	tagNode.args = args

	return tagNode, nil
}
