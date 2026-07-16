package eino

import (
	"context"
	"errors"
	"sync"

	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"kandaoni.com/anqicms/config"
)

type Configs struct {
	Configs   []*Config `json:"configs"`
	LastModel string    `json:"last_model"`
}

// Config holds the configuration for Eino AI integration.
type Config struct {
	APIKey          string  `json:"api_key"`
	Model           string  `json:"model"`
	BaseURL         string  `json:"base_url"`
	MaxTokens       int     `json:"max_tokens"`
	EnableReasoning bool    `json:"enable_reasoning"`
	Temperature     float64 `json:"temperature"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
	MaxRetries      int     `json:"max_retries"`
	Name            string  `json:"name"`
}

// Global instance
var (
	globalConfig    *Config
	globalConfigMu  sync.RWMutex
	globalClient    *einoOpenAI.ChatModel
	globalClientMu  sync.RWMutex
	clientTokenSnap string // 记录创建 client 时使用的 token，用于检测变更
	isOfficialCfg   bool   // 标记当前是否为官方配置模式
)

// GlobalConfig returns the global Eino configuration.
func GlobalConfig() *Config {
	globalConfigMu.RLock()
	defer globalConfigMu.RUnlock()
	return globalConfig
}

func SetOfficialConfig(modelName string) error {
	// official model = "anqi-flash"|"anqi-pro"
	if modelName != "anqi-flash" && modelName != "anqi-pro" {
		// 设置默认模型
		modelName = "anqi-flash"
	}
	cfg := &Config{
		Name:    "official",
		Model:   modelName,
		BaseURL: "https://auth.anqicms.com/auth/v1",
		APIKey:  config.AnqiUser.Token, // 初始值，InitClient 时会实时读取
	}

	return SetGlobalConfig(cfg)
}

// SetGlobalConfig sets the global Eino configuration.
func SetGlobalConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	if cfg.APIKey == "" {
		return errors.New("API key is required")
	}
	if cfg.Model == "" {
		return errors.New("Model is required")
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 8192
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 120
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	cfg.EnableReasoning = true

	globalConfigMu.Lock()
	globalConfig = cfg
	isOfficialCfg = (cfg.Name == "official")
	globalConfigMu.Unlock()

	return InitClient()
}

// InitClient creates the global Eino ChatModel client based on configuration.
// For official config, it always reads the latest token from config.AnqiUser.Token.
func InitClient() error {
	globalClientMu.Lock()
	defer globalClientMu.Unlock()

	if globalConfig == nil {
		return errors.New("config not initialized")
	}

	// 官方模式下实时读取最新 token
	apiKey := globalConfig.APIKey
	if globalConfig.Name == "official" {
		apiKey = config.AnqiUser.Token
	}
	clientTokenSnap = apiKey

	if globalConfig.EnableReasoning {
		// deepseek-reasoner does not support Temperature
		model := globalConfig.Model
		if model == "" {
			model = "deepseek-v4-flash"
		}
		baseURL := globalConfig.BaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com"
		}

		cli, err := einoOpenAI.NewChatModel(context.Background(), &einoOpenAI.ChatModelConfig{
			BaseURL:     baseURL,
			APIKey:      apiKey,
			Model:       model,
			MaxTokens:   &globalConfig.MaxTokens,
			ExtraFields: map[string]any{"deepseek_reasoning": true},
		})
		if err != nil {
			return err
		}

		globalClient = cli
		return nil
	}

	baseURL := globalConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	model := globalConfig.Model
	if model == "" {
		model = "deepseek-v4-flash"
	}
	temperature := float32(globalConfig.Temperature)

	cli, err := einoOpenAI.NewChatModel(context.Background(), &einoOpenAI.ChatModelConfig{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       model,
		MaxTokens:   &globalConfig.MaxTokens,
		Temperature: &temperature,
	})
	if err != nil {
		return err
	}

	globalClient = cli
	return nil
}

// GetClient returns the global Eino ChatModel client.
// If the AnqiUser token has changed since the client was created,
// it automatically reinitializes the client with the new token.
func GetClient() (*einoOpenAI.ChatModel, error) {
	globalClientMu.RLock()
	client := globalClient
	snap := clientTokenSnap
	globalClientMu.RUnlock()

	if client == nil {
		return nil, errors.New("client not initialized, call SetGlobalConfig or InitClient first")
	}

	// 官方模式下检查 token 是否已变更，自动重初始化
	if isOfficialCfg && snap != config.AnqiUser.Token {
		globalClientMu.Lock()
		// Double-check
		if clientTokenSnap != config.AnqiUser.Token {
			if err := InitClient(); err != nil {
				globalClientMu.Unlock()
				return nil, err
			}
		}
		client = globalClient
		globalClientMu.Unlock()
	}

	return client, nil
}

// GenerateText uses Eino ChatModel to generate text completion.
func GenerateText(ctx context.Context, prompt string, options ...GenerateOption) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	cfg := applyOptions(options)

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	msg, err := client.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	result := msg.Content
	if cfg.Stream {
		// For streaming, we still do a full generate since we concatenate
		// This path is kept for API compatibility
	}

	return result, nil
}

// GenerateStructured uses Eino ChatModel to generate structured JSON output.
func GenerateStructured[T any](ctx context.Context, prompt string, systemPrompt string) (*T, error) {
	client, err := GetClient()
	if err != nil {
		return nil, err
	}

	var messages []*schema.Message
	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}
	messages = append(messages, schema.UserMessage(prompt))

	msg, err := client.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}

	var parsed T
	if err := parseJSON(msg.Content, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

// compose graph node types for content management
type GraphInput struct {
	Action   string `json:"action"` // "create", "update", "suggest"
	Title    string `json:"title"`
	Content  string `json:"content"`
	Category string `json:"category"`
	Keywords string `json:"keywords"`
	Language string `json:"language"`
}

type GraphOutput struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	Keywords     string   `json:"keywords"`
	Description  string   `json:"description"`
	Suggestions  []string `json:"suggestions"`
	SEO          string   `json:"seo"`
	ErrorMessage string   `json:"error_message,omitempty"`
}

// GenerateOption allows configuring GenerateText calls.
type GenerateOption func(*generateConfig)

type generateConfig struct {
	Stream    bool
	MaxTokens int
}

func applyOptions(opts []GenerateOption) *generateConfig {
	cfg := &generateConfig{Stream: false}
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}

func WithStream() GenerateOption {
	return func(c *generateConfig) { c.Stream = true }
}

func WithMaxTokens(n int) GenerateOption {
	return func(c *generateConfig) { c.MaxTokens = n }
}

func parseJSON(s string, v any) error {
	if s == "" {
		return nil
	}
	return nil
}
