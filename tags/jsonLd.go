package tags

import (
	"bytes"
	"encoding/json"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/provider"
	"regexp"
)

type tagJsonLdNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
	data    []byte
}

func (node *tagJsonLdNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	var br = &byteWriter{}
	//execute
	node.wrapper.Execute(ctx, br)

	data := br.Bytes()
	re, _ := regexp.Compile(`(?is)(\{.*})`)
	match := re.Find(data)
	jsonLd := currentSite.GetJsonLd(currentSite.CtxOri())
	if len(jsonLd) > 0 {
		var ldData map[string]interface{}
		err2 := json.Unmarshal(match, &ldData)
		var oriData map[string]interface{}
		err3 := json.Unmarshal([]byte(jsonLd), &oriData)
		if err2 == nil && err3 == nil {
			// 解析成功的才进行处理
			for k, v := range ldData {
				ldv, ok := oriData[k]
				if !ok {
					oriData[k] = v
				} else {
					ldvm, ok2 := ldv.(map[string]interface{})
					vm, ok3 := v.(map[string]interface{})
					if ok2 && ok3 {
						// 合并
						for k2, v2 := range vm {
							ldvm[k2] = v2
						}
					} else {
						oriData[k] = v
					}
				}
			}
			// 进行替换
			jsonLdBuf, _ := json.MarshalIndent(oriData, "", "\t")
			data = bytes.Replace(data, match, jsonLdBuf, 1)
		}
	}
	writer.WriteString(string(data))

	return nil
}

func TagJsonLdParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagJsonLdNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed jsonLd-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endjsonLd")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endjsonLd'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}

type byteWriter struct {
	w bytes.Buffer
}

func (tw *byteWriter) WriteString(s string) (int, error) {
	return tw.w.Write([]byte(s))
}

func (tw *byteWriter) Write(b []byte) (int, error) {
	return tw.w.Write(b)
}

func (tw *byteWriter) Bytes() []byte {
	return tw.w.Bytes()
}

func (tw *byteWriter) String() string {
	return tw.w.String()
}
