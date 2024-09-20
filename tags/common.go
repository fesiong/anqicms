package tags

import "github.com/flosch/pongo2/v6"

func parseArgs(args map[string]pongo2.IEvaluator, ctx *pongo2.ExecutionContext) (map[string]*pongo2.Value, *pongo2.Error) {
	parsedArgs := map[string]*pongo2.Value{}
	for key, value := range args {
		val, err := value.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
		parsedArgs[key] = val
	}

	return parsedArgs, nil
}

func parseWith(arguments *pongo2.Parser) (map[string]pongo2.IEvaluator, *pongo2.Error) {
	args := make(map[string]pongo2.IEvaluator)
	// After having parsed the name we're gonna parse the with options
	if arguments.Match(pongo2.TokenIdentifier, "with") != nil {
		for arguments.Remaining() > 0 {
			// We have at least one key=expr pair (because of starting "with")
			keyToken := arguments.MatchType(pongo2.TokenIdentifier)
			if keyToken == nil {
				return nil, arguments.Error("Expected an identifier", nil)
			}
			if arguments.Match(pongo2.TokenSymbol, "=") == nil {
				return nil, arguments.Error("Expected '='.", nil)
			}
			valueExpr, err := arguments.ParseExpression()
			if err != nil {
				return nil, arguments.Error("Can not parse with args.", keyToken)
			}

			args[keyToken.Val] = valueExpr
		}
	}

	return args, nil
}
