//go:build ignore

package eino

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
)

// ContentAgent generates and optimizes content using AI.
type ContentAgent struct {
	client *model.AnyModel
	system prompt.Prompt
}

// NewContentAgent creates a new ContentAgent.
func NewContentAgent(client *model.AnyModel) *ContentAgent {
	return &ContentAgent{
		client: client,
		system: &TemplateSystemPrompt{},
	}
}

// GenerateArticle generates an article based on title and keywords.
func (a *ContentAgent) GenerateArticle(ctx context.Context, title, keywords, category string) (*GraphOutput, error) {
	prompt := fmt.Sprintf(`
Generate a high-quality article based on:
- Title: %s
- Keywords: %s
- Category: %s

Requirements:
1. Write a compelling title (SEO optimized)
2. Create comprehensive content (>1000 characters)
3. Generate keywords and description for SEO
4. Return structured JSON

JSON format:
{
  "title": "SEO optimized title",
  "content": "# Article Content\n\nFull article here...",
  "keywords": "keyword1, keyword2, keyword3",
  "description": "Brief description for meta tag",
  "seo": "SEO analysis and suggestions"
}

Language: Chinese
`, title, keywords, category)

	output, err := GenerateStructured[GraphOutput](ctx, prompt, "You are a professional content writer and SEO expert.")
	if err != nil {
		return &GraphOutput{
			Success:        false,
			ErrorMessage:   err.Error(),
			Title:          title,
		}, err
	}

	output.Success = true
	return output, nil
}

// OptimizeContent optimizes existing content for SEO and readability.
func (a *ContentAgent) OptimizeContent(ctx context.Context, content, title string) (*GraphOutput, error) {
	prompt := fmt.Sprintf(`
Optimize the following article content for SEO and readability:

Title: %s
Content: %s

Provide optimized JSON with:
1. Improved title (if needed)
2. Optimized content with better structure
3. Enhanced keywords
4. Better description
5. SEO improvement suggestions

JSON format:
{
  "title": "optimized title",
  "content": "optimized content",
  "keywords": "keyword1, keyword2",
  "description": "optimized description",
  "suggestions": ["suggestion1", "suggestion2"],
  "seo": "detailed SEO analysis"
}
`, title, content)

	output, err := GenerateStructured[GraphOutput](ctx, prompt, "You are an SEO expert and content optimizer.")
	if err != nil {
		return &GraphOutput{
			Success:        false,
			ErrorMessage:   err.Error(),
		}, err
	}

	output.Success = true
	return output, nil
}

// ClassifyContent classifies content into appropriate categories.
func (a *ContentAgent) ClassifyContent(ctx context.Context, title, content string) (*GraphOutput, error) {
	prompt := fmt.Sprintf(`
Classify this content into appropriate categories:

Title: %s
Content: %s

Return JSON format:
{
  "success": true,
  "category": "recommended category",
  "keywords": "auto-generated keywords",
  "description": "auto-generated description",
  "suggestions": ["suggestion1"]
}
`, title, content)

	output, err := GenerateStructured[GraphOutput](ctx, prompt, "You are a content classification expert.")
	if err != nil {
		return &GraphOutput{
			Success:        false,
			ErrorMessage:   err.Error(),
		}, err
	}

	output.Success = true
	return output, nil
}

// TemplateSystemPrompt is a simple prompt template.
type TemplateSystemPrompt struct{}

func (p *TemplateSystemPrompt) Format(ctx context.Context, variables map[string]any) (string, error) {
	prompt := `You are an AI assistant for content management. Your tasks include:
1. Writing high-quality articles
2. Optimizing content for SEO
3. Classifying content into categories
4. Generating metadata (titles, descriptions, keywords)

Please respond in a structured and helpful manner.`

	if variables != nil {
		for k, v := range variables {
			prompt = fmt.Sprintf(prompt, v)
			break // simple replacement
		}
	}
	return prompt, nil
}
