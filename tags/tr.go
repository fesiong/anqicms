package tags

import (
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/provider"
)

// tagTrNode 翻译
type tagTrNode struct {
	args []pongo2.IEvaluator
	key  string
}

func (node *tagTrNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	var args []interface{}
	for _, value := range node.args {
		val, err := value.Evaluate(ctx)
		if err != nil {
			return err
		}
		args = append(args, val.Interface())
	}

	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		writer.WriteString(node.key)
		return nil
	}

	writer.WriteString(currentSite.TplTr(node.key, args...))

	return nil
}

func TagTrParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagTrNode{
		args: []pongo2.IEvaluator{},
	}

	if arguments.Remaining() > 0 {
		arg := arguments.Current()
		arguments.Consume()
		tagNode.key = arg.Val
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
