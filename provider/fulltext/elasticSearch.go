package fulltext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"kandaoni.com/anqicms/config"
)

type ElasticSearchService struct {
	config    *config.PluginFulltextConfig
	indexName string
	baseUrl   string
	username  string
	password  string
	client    *http.Client
}

type ElasticIndexProperty struct {
	Type          string `json:"type"`
	Index         bool   `json:"index"`
	Store         bool   `json:"store,omitempty"`
	Sortable      bool   `json:"sortable,omitempty"`
	Highlightable bool   `json:"highlightable,omitempty"`
	Aggregatable  bool   `json:"aggregatable,omitempty"`
}

func NewElasticSearchService(cfg *config.PluginFulltextConfig, indexName string) (Service, error) {
	baseUrl := strings.TrimRight(cfg.EngineUrl, "/")
	s := &ElasticSearchService{
		config:    cfg,
		indexName: indexName,
		baseUrl:   baseUrl,
		username:  cfg.EngineUser,
		password:  cfg.EnginePass,
		client:    http.DefaultClient,
	}

	return s, nil
}

// doRequest 发送 HTTP 请求到 ES，返回响应 body
func (s *ElasticSearchService) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	url := s.baseUrl + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, err
	}
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

func (s *ElasticSearchService) Index(body interface{}) error {
	ctx := context.Background()

	// 检查索引是否存在
	_, status, err := s.doRequest(ctx, "HEAD", "/"+s.indexName, nil)
	if err != nil {
		log.Printf("Error when calling Index.Exists: %v\n", err)
		return err
	}

	if status == 404 {
		// 创建索引
		mapping := map[string]interface{}{
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"type": map[string]string{
						"type": "keyword",
					},
					"module_id": map[string]string{
						"type": "integer",
					},
					"title": map[string]string{
						"type": "text",
					},
					"keywords": map[string]string{
						"type": "text",
					},
					"description": map[string]string{
						"type": "text",
					},
					"content": map[string]string{
						"type": "text",
					},
				},
			},
		}
		bodyBytes, _ := json.Marshal(mapping)
		respBody, status, err := s.doRequest(ctx, "PUT", "/"+s.indexName, bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("Error when calling Index.Create: %v\n", err)
			return err
		}
		if status >= 400 {
			log.Printf("Error response from Index.Create (status=%d): %s\n", status, string(respBody))
		}
	}

	return nil
}

func (s *ElasticSearchService) Create(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	bodyBytes, _ := json.Marshal(doc)
	// PUT /{index}/_doc/{id}
	respBody, status, err := s.doRequest(context.Background(), "PUT",
		fmt.Sprintf("/%s/_doc/%s", s.indexName, docId),
		bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("Error when calling Document.Index: %v\n", err)
		return err
	}
	if status >= 400 {
		log.Printf("Error response from Document.Index (status=%d): %s\n", status, string(respBody))
	}

	return nil
}

func (s *ElasticSearchService) Update(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	updateBody := map[string]interface{}{
		"doc": doc,
	}
	bodyBytes, _ := json.Marshal(updateBody)
	// POST /{index}/_update/{id}
	respBody, status, err := s.doRequest(context.Background(), "POST",
		fmt.Sprintf("/%s/_update/%s", s.indexName, docId),
		bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("Error when calling Document.Update: %v\n", err)
		return err
	}
	if status >= 400 {
		log.Printf("Error response from Document.Update (status=%d): %s\n", status, string(respBody))
	}

	return nil
}

func (s *ElasticSearchService) Delete(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	// DELETE /{index}/_doc/{id}
	respBody, status, err := s.doRequest(context.Background(), "DELETE",
		fmt.Sprintf("/%s/_doc/%s", s.indexName, docId), nil)
	if err != nil {
		log.Printf("Error when calling Document.Delete: %v\n", err)
		return err
	}
	if status >= 400 {
		log.Printf("Error response from Document.Delete (status=%d): %s\n", status, string(respBody))
	}

	return nil
}

func (s *ElasticSearchService) Bulk(docs []TinyArchive) error {
	buff := new(bytes.Buffer)
	for _, v := range docs {
		docId := v.GetId()
		docIdStr := strconv.FormatInt(docId, 10)
		buff.WriteString(fmt.Sprintf(`{ "index" : { "_index" : "%s", "_id" : "%s" } }`+"\n", s.indexName, docIdStr))
		buf, _ := json.Marshal(v)
		buff.Write(buf)
		buff.WriteString("\n")
	}
	// POST /{index}/_bulk
	respBody, status, err := s.doRequest(context.Background(), "POST",
		fmt.Sprintf("/%s/_bulk", s.indexName), buff)
	if err != nil {
		log.Printf("Error when calling Document.Bulk: %v\n", err)
		return err
	}
	if status >= 400 {
		log.Printf("Error response from Document.Bulk (status=%d): %s\n", status, string(respBody))
	}

	return nil
}

// esSearchHit 解析 ES 搜索命中的字段
type esSearchHit struct {
	Index  string          `json:"_index"`
	Id     string          `json:"_id"`
	Score  *float64        `json:"_score"`
	Source json.RawMessage `json:"_source"`
}

type esSearchHits struct {
	Total    *esTotal      `json:"total"`
	MaxScore *float64      `json:"max_score"`
	Hits     []esSearchHit `json:"hits"`
}

type esTotal struct {
	Value int64 `json:"value"`
}

type esSearchResult struct {
	Took int          `json:"took"`
	Hits esSearchHits `json:"hits"`
}

func (s *ElasticSearchService) Search(keyword string, moduleId uint, page int, pageSize int) ([]TinyArchive, int64, error) {
	if page < 1 {
		page = 1
	}

	// 构建 bool query
	matchFields := []string{"title", "keywords", "description", "content"}
	shouldClauses := make([]map[string]interface{}, 0, len(matchFields))
	for _, field := range matchFields {
		shouldClauses = append(shouldClauses, map[string]interface{}{
			"match": map[string]interface{}{
				field: map[string]interface{}{
					"query": keyword,
				},
			},
		})
	}

	var query map[string]interface{}
	if moduleId > 0 {
		query = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"bool": map[string]interface{}{
							"should":               shouldClauses,
							"minimum_should_match": 1,
						},
					},
					{
						"term": map[string]interface{}{
							"module_id": moduleId,
						},
					},
				},
			},
		}
	} else {
		query = map[string]interface{}{
			"bool": map[string]interface{}{
				"should":               shouldClauses,
				"minimum_should_match": 1,
			},
		}
	}

	searchBody := map[string]interface{}{
		"query": query,
	}
	bodyBytes, _ := json.Marshal(searchBody)

	from := pageSize * (page - 1)

	// GET /{index}/_search
	path := fmt.Sprintf("/%s/_search?from=%d&size=%d", s.indexName, from, pageSize)
	respBody, status, err := s.doRequest(context.Background(), "GET", path, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("Error when calling Search: %v\n", err)
		return nil, 0, err
	}
	if status >= 400 {
		log.Printf("Error response from Search (status=%d): %s\n", status, string(respBody))
		return nil, 0, nil
	}

	var result esSearchResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("Error decoding search response: %v\n", err)
		return nil, 0, err
	}

	// 归一化分值
	var maxScore float64
	if result.Hits.MaxScore != nil {
		maxScore = *result.Hits.MaxScore
	} else {
		for _, hit := range result.Hits.Hits {
			if hit.Score != nil && *hit.Score > maxScore {
				maxScore = *hit.Score
			}
		}
	}

	var docs = make([]TinyArchive, 0, pageSize)
	for _, hit := range result.Hits.Hits {
		id, _ := strconv.ParseInt(hit.Id, 10, 64)
		doc := TinyArchive{}
		_ = json.Unmarshal(hit.Source, &doc)
		doc.Id, doc.Type = GetId(id)
		// ContainLength 过滤
		if s.config.ContainLength > 0 {
			if !containsByLength(keyword, s.config.ContainLength, doc.Title, doc.Description, doc.Content) {
				continue
			}
		}
		// RankingScore 过滤
		if s.config.RankingScore > 0 && maxScore > 0 && hit.Score != nil {
			norm := *hit.Score / maxScore
			if norm < float64(s.config.RankingScore)/100.0 {
				continue
			}
		}
		docs = append(docs, doc)
	}
	total := int64(len(docs))

	return docs, total, nil
}

func (s *ElasticSearchService) Close() {
}

func (s *ElasticSearchService) Flush() {
}

// containsByLength 判断在任一字段中是否包含关键字；当关键字长度小于等于阈值时要求完整包含；
// 当关键字长度大于阈值时，要求至少包含任意连续阈值长度的子串。
func containsByLength(keyword string, length int, fields ...string) bool {
	if length <= 0 {
		return true
	}
	joined := strings.Join(fields, " ")
	kr := []rune(keyword)
	if utf8.RuneCountInString(keyword) <= length {
		return strings.Contains(joined, keyword)
	}
	for i := 0; i <= len(kr)-length; i++ {
		sub := string(kr[i : i+length])
		if strings.Contains(joined, sub) {
			return true
		}
	}
	return false
}
