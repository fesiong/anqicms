package tags

import (
	"github.com/flosch/pongo2/v6"
)

type tagSetNode struct {
	name       string
	expression pongo2.IEvaluator
	global     pongo2.IEvaluator
}

func (node *tagSetNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	// Evaluate expression
	value, err := node.expression.Evaluate(ctx)
	if err != nil {
		return err
	}

	if node.global != nil {
		ctx.Public[node.name] = value
	} else {
		ctx.Private[node.name] = value
	}

	return nil
}

func TagSetParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	node := &tagSetNode{}

	// Parse variable name
	typeToken := arguments.MatchType(pongo2.TokenIdentifier)
	if typeToken == nil {
		return nil, arguments.Error("Expected an identifier.", nil)
	}
	node.name = typeToken.Val

	if arguments.Match(pongo2.TokenSymbol, "=") == nil {
		return nil, arguments.Error("Expected '='.", nil)
	}

	// Variable expression
	keyExpression, err := arguments.ParseExpression()
	if err != nil {
		return nil, err
	}
	node.expression = keyExpression

	if arguments.Remaining() > 0 {
		// 仅接受global参数
		keyToken := arguments.MatchType(pongo2.TokenIdentifier)
		if keyToken == nil {
			return nil, arguments.Error("Expected an identifier", nil)
		}
		if keyToken.Val != "global" {
			return nil, arguments.Error("Expected 'global'.", nil)
		}
		if arguments.Match(pongo2.TokenSymbol, "=") == nil {
			return nil, arguments.Error("Expected '='.", nil)
		}
		valueExpr, err := arguments.ParseExpression()
		if err != nil {
			return nil, arguments.Error("Can not parse with args.", keyToken)
		}
		node.global = valueExpr
	}

	// Remaining arguments
	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed 'set'-tag arguments.", nil)
	}

	return node, nil
}
