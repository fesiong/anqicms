package fulltext

import (
	"bytes"
	"context"
	"encoding/json"
	es8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"kandaoni.com/anqicms/config"
	"log"
	"strconv"
)

type ElasticSearchService struct {
	config    *config.PluginFulltextConfig
	apiClient *es8.TypedClient
	indexName string
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
	es8cfg := es8.Config{
		Addresses: []string{
			cfg.EngineUrl,
		},
		Username: cfg.EngineUser,
		Password: cfg.EnginePass,
	}
	es, err := es8.NewTypedClient(es8cfg)
	if err != nil {
		return nil, err
	}

	s := &ElasticSearchService{
		config:    cfg,
		indexName: indexName,
		apiClient: es,
	}

	return s, nil
}

func (s *ElasticSearchService) Index(body interface{}) error {
	mappings := types.TypeMapping{
		Properties: map[string]types.Property{
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
	}

	ok, err := s.apiClient.Indices.Exists(s.indexName).Do(context.TODO())
	if err != nil {
		log.Printf("Error when calling `Index.Exists``: %v\n", err)
		return err
	}
	if !ok {
		res, err := s.apiClient.Indices.Create(s.indexName).Mappings(&mappings).Do(context.TODO())
		if err != nil {
			log.Printf("Error when calling `Index.Create``: %v\n", err)
			log.Printf("Full HTTP response: %v\n", res)
			return err
		}
	}

	return nil
}

func (s *ElasticSearchService) Create(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	r, err := s.apiClient.Index(s.indexName).Id(docId).Request(doc).Do(context.TODO())
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ElasticSearchService) Update(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	r, err := s.apiClient.Update(s.indexName, docId).Doc(doc).Do(context.TODO())
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ElasticSearchService) Delete(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	r, err := s.apiClient.Delete(s.indexName, docId).Do(context.TODO())
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ElasticSearchService) Bulk(docs []TinyArchive) error {
	buff := new(bytes.Buffer)
	// { "index" : { "_index" : "test", "_id" : "1" } }
	for _, v := range docs {
		docId := v.GetId()
		docIdStr := strconv.FormatInt(docId, 10)
		buff.WriteString("{ \"index\" : { \"_index\" : \"" + s.indexName + "\", \"_id\" : \"" + docIdStr + "\" } }\n")
		buf, _ := json.Marshal(v)
		buff.Write(buf)
		buff.WriteString("\n")
	}
	r, err := s.apiClient.Bulk().Index(s.indexName).Raw(buff).Do(context.TODO())
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ElasticSearchService) Search(keyword string, moduleId uint, page int, pageSize int) ([]TinyArchive, int64, error) {
	if page < 1 {
		page = 1
	}

	var query types.Query

	if moduleId > 0 {
		query = types.Query{
			Bool: &types.BoolQuery{
				Must: []types.Query{
					{
						Bool: &types.BoolQuery{
							Should: []types.Query{
								{Match: map[string]types.MatchQuery{"title": {Query: keyword}}},
								{Match: map[string]types.MatchQuery{"keywords": {Query: keyword}}},
								{Match: map[string]types.MatchQuery{"description": {Query: keyword}}},
								{Match: map[string]types.MatchQuery{"content": {Query: keyword}}},
							},
							MinimumShouldMatch: 1,
						},
					},
					{
						Term: map[string]types.TermQuery{
							"module_id": {Value: moduleId},
						},
					},
				},
			},
		}
	} else {
		query = types.Query{
			Bool: &types.BoolQuery{
				Should: []types.Query{
					{Match: map[string]types.MatchQuery{"title": {Query: keyword}}},
					{Match: map[string]types.MatchQuery{"keywords": {Query: keyword}}},
					{Match: map[string]types.MatchQuery{"description": {Query: keyword}}},
					{Match: map[string]types.MatchQuery{"content": {Query: keyword}}},
				},
				MinimumShouldMatch: 1,
			},
		}
	}

	from := pageSize * (page - 1)
	resp, err := s.apiClient.Search().Index(s.indexName).Query(&query).From(from).Size(pageSize).Do(context.TODO())
	if err != nil {
		log.Printf("Error when calling `SearchApi.Search``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", resp)
		return nil, 0, err
	}

	var docs = make([]TinyArchive, 0, pageSize)
	for _, hit := range resp.Hits.Hits {
		id, _ := strconv.ParseInt(*hit.Id_, 10, 64)
		doc := TinyArchive{}
		_ = json.Unmarshal(hit.Source_, &doc)
		doc.Id, doc.Type = GetId(id)
		docs = append(docs, doc)
	}
	total := resp.Hits.Total.Value

	return docs, total, nil
}

func (s *ElasticSearchService) Close() {
}

func (s *ElasticSearchService) Flush() {
}
