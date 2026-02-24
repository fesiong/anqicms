package fulltext

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/meilisearch/meilisearch-go"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
)

type MeiliSearchService struct {
	config    *config.PluginFulltextConfig
	apiClient meilisearch.ServiceManager
	indexName string
}

func NewMeiliSearchService(cfg *config.PluginFulltextConfig, indexName string) (Service, error) {
	meili := &MeiliSearchService{
		config:    cfg,
		indexName: indexName,
		apiClient: meilisearch.New(cfg.EngineUrl, meilisearch.WithAPIKey(cfg.EnginePass)),
	}

	_, err := meili.apiClient.GetStats()
	if err != nil {
		return nil, err
	}

	return meili, nil
}

func (s *MeiliSearchService) Index(body interface{}) error {
	// features
	s.apiClient.ExperimentalFeatures().SetContainsFilter(true).Update()

	index := s.apiClient.Index(s.indexName)

	task0, err := index.UpdateFilterableAttributes(&[]interface{}{"module_id", "title"})
	if err != nil {
		log.Println("配置可搜索属性失败: ", err)
		return err
	}
	task, err := index.UpdateSearchableAttributes(&[]string{"title", "keywords", "description", "content"})
	if err != nil {
		log.Println("配置可搜索属性失败: ", err)
		return err
	}
	for {
		time.Sleep(1 * time.Second)
		task2, _ := s.apiClient.GetTask(task.TaskUID)
		task3, _ := s.apiClient.GetTask(task0.TaskUID)
		log.Printf("task status %v", task2.Status)
		if task2.Status == "succeeded" && task3.Status == "succeeded" {
			break
		}
	}

	fmt.Println("已成功配置搜索属性。")

	return nil
}

func (s *MeiliSearchService) Create(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)
	newTitle := strings.ReplaceAll(doc.Title, "-", "")
	newTitle = strings.ReplaceAll(newTitle, "/", "")
	newTitle = strings.ReplaceAll(newTitle, " ", "")
	doc.Title = doc.Title + " " + newTitle
	data := library.StructToMap(doc)
	data["id"] = docId
	documents := []map[string]interface{}{
		data,
	}
	task, err := s.apiClient.Index(s.indexName).AddDocuments(documents, nil)
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", task)
		return err
	}

	return nil
}

func (s *MeiliSearchService) Update(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)
	newTitle := strings.ReplaceAll(doc.Title, "-", "")
	newTitle = strings.ReplaceAll(newTitle, "/", "")
	newTitle = strings.ReplaceAll(newTitle, " ", "")
	doc.Title = doc.Title + " " + newTitle
	data := library.StructToMap(doc)
	data["id"] = docId
	documents := []map[string]interface{}{
		data,
	}
	task, err := s.apiClient.Index(s.indexName).UpdateDocuments(documents, nil)
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", task)
		return err
	}

	return nil
}

func (s *MeiliSearchService) Delete(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	task, err := s.apiClient.Index(s.indexName).DeleteDocument(docId, nil)
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", task)
		return err
	}

	return nil
}

func (s *MeiliSearchService) Bulk(docs []TinyArchive) error {
	var data []map[string]interface{}
	for _, v := range docs {
		docId := v.GetId()
		newTitle := strings.ReplaceAll(v.Title, "-", "")
		newTitle = strings.ReplaceAll(newTitle, "/", "")
		newTitle = strings.ReplaceAll(newTitle, " ", "")
		v.Title = v.Title + " " + newTitle
		item := library.StructToMap(v)
		// docId
		item["id"] = strconv.FormatInt(docId, 10)

		data = append(data, item)
	}
	task, err := s.apiClient.Index(s.indexName).AddDocuments(data, nil)
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", task)
		return err
	}

	return nil
}

func (s *MeiliSearchService) Search(keyword string, moduleId uint, page int, pageSize int) ([]TinyArchive, int64, error) {
	if page < 1 {
		page = 1
	}

	query := &meilisearch.SearchRequest{
		Limit:  int64(pageSize),
		Offset: int64((page - 1) * pageSize),
	}
	if s.config.RankingScore > 0 {
		query.RankingScoreThreshold = float64(s.config.RankingScore) / 100
	}
	var queryFilter []string
	if moduleId > 0 {
		queryFilter = append(queryFilter, fmt.Sprintf("module_id = %d", moduleId)) // 过滤 moduleId
	}
	if s.config.ContainLength > 0 {
		// 匹配搜索词的开头
		contain := keyword
		if utf8.RuneCountInString(contain) > s.config.ContainLength {
			contain = string([]rune(contain)[:s.config.ContainLength])
		}
		// "title STARTS WITH 'keyword'")
		queryFilter = append(queryFilter, fmt.Sprintf("title CONTAINS '%s'", contain))
	}
	if len(queryFilter) > 0 {
		query.Filter = queryFilter
	}

	resp, err := s.apiClient.Index(s.indexName).Search(keyword, query)
	if err != nil {
		log.Printf("Error when calling `SearchApi.Search``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", resp)
		return nil, 0, err
	}
	var docs = make([]TinyArchive, 0, pageSize)
	for _, hit := range resp.Hits {
		var idStr string
		json.Unmarshal(hit["id"], &idStr)
		id, _ := strconv.ParseInt(idStr, 10, 64)
		doc := TinyArchive{
			Id: id,
		}
		if _, ok := hit["type"]; ok {
			json.Unmarshal(hit["type"], &doc.Type)
		}
		if _, ok := hit["title"]; ok {
			json.Unmarshal(hit["title"], &doc.Title)
		}
		if _, ok := hit["description"]; ok {
			json.Unmarshal(hit["description"], &doc.Description)
		}
		if _, ok := hit["content"]; ok {
			json.Unmarshal(hit["content"], &doc.Content)
		}
		if _, ok := hit["keywords"]; ok {
			json.Unmarshal(hit["keywords"], &doc.Keywords)
		}
		if _, ok := hit["module_id"]; ok {
			json.Unmarshal(hit["module_id"], &doc.ModuleId)
		}
		doc.Id, doc.Type = GetId(id)
		docs = append(docs, doc)
	}
	total := resp.EstimatedTotalHits

	return docs, total, nil
}

func (s *MeiliSearchService) Close() {
	// nothing to do
}

func (s *MeiliSearchService) Flush() {
	// nothing to do
}
