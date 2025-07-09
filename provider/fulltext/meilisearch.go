package fulltext

import (
	"fmt"
	"github.com/meilisearch/meilisearch-go"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"strconv"
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
	// no need to add first

	return nil
}

func (s *MeiliSearchService) Create(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)
	data := library.StructToMap(doc)
	data["id"] = docId
	documents := []map[string]interface{}{
		data,
	}

	task, err := s.apiClient.Index(s.indexName).AddDocuments(documents, "id")
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
	data := library.StructToMap(doc)
	data["id"] = docId
	documents := []map[string]interface{}{
		data,
	}

	task, err := s.apiClient.Index(s.indexName).UpdateDocuments(documents, "id")
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

	task, err := s.apiClient.Index(s.indexName).DeleteDocument(docId)
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
		item := library.StructToMap(v)
		// docId
		item["id"] = strconv.FormatInt(docId, 10)

		data = append(data, item)
	}

	task, err := s.apiClient.Index(s.indexName).AddDocuments(data, "id")
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
	if moduleId > 0 {
		query.Filter = fmt.Sprintf("module_id = %d", moduleId) // 过滤 moduleId
	}

	resp, err := s.apiClient.Index(s.indexName).Search(keyword, query)
	if err != nil {
		log.Printf("Error when calling `SearchApi.Search``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", resp)
		return nil, 0, err
	}

	var docs = make([]TinyArchive, 0, pageSize)
	for _, hit := range resp.Hits {
		item, ok := hit.(map[string]interface{})
		if ok {
			tmpId, _ := item["id"].(string)
			id, _ := strconv.ParseInt(tmpId, 10, 64)
			doc := TinyArchive{}
			_ = library.MapToStruct(item, &doc)
			doc.Id, doc.Type = GetId(id)
			docs = append(docs, doc)
		}
	}
	total := resp.TotalHits

	return docs, total, nil
}

func (s *MeiliSearchService) Close() {
	// nothing to do
}

func (s *MeiliSearchService) Flush() {
	// nothing to do
}
