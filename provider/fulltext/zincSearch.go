package fulltext

import (
	"context"
	"errors"
	client "github.com/zinclabs/sdk-go-zincsearch"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"strconv"
)

type ZincSearchService struct {
	config    *config.PluginFulltextConfig
	apiClient *client.APIClient
	ctx       context.Context
	indexName string
}

type ZincIndexProperty struct {
	Type          string `json:"type"`
	Index         bool   `json:"index"`
	Store         bool   `json:"store,omitempty"`
	Sortable      bool   `json:"sortable,omitempty"`
	Highlightable bool   `json:"highlightable,omitempty"`
	Aggregatable  bool   `json:"aggregatable,omitempty"`
}

func NewZincSearchService(cfg *config.PluginFulltextConfig, indexName string) (Service, error) {
	configuration := client.NewConfiguration()
	configuration.Servers = client.ServerConfigurations{
		client.ServerConfiguration{
			URL:         cfg.EngineUrl,
			Description: "ZincSearch",
		},
	}
	ctx := context.WithValue(context.Background(), client.ContextBasicAuth, client.BasicAuth{
		UserName: cfg.EngineUser,
		Password: cfg.EnginePass,
	})

	zinc := &ZincSearchService{
		config:    cfg,
		ctx:       ctx,
		indexName: indexName,
		apiClient: client.NewAPIClient(configuration),
	}
	login := *client.NewAuthLoginRequest() // AuthLoginRequest | Login credentials

	_, r, err := zinc.apiClient.User.Login(ctx).Login(login).Execute()
	if err != nil {
		log.Printf("Error when calling `User.Login``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, errors.New("login error")
	}

	return zinc, nil
}

func (s *ZincSearchService) Index(body interface{}) error {
	//var analyzer = "gse_standard"
	data := client.MetaIndexSimple{
		Name: &s.indexName,
		Mappings: map[string]interface{}{
			"properties": map[string]ZincIndexProperty{
				"type": {
					Type:         "keyword",
					Index:        true,
					Sortable:     true,
					Aggregatable: true,
				},
				"module_id": {
					Type:         "numeric",
					Index:        true,
					Sortable:     true,
					Aggregatable: true,
				},
				"title": {
					Type:          "text",
					Index:         true,
					Store:         true,
					Highlightable: true,
				},
				"keywords": {
					Type:          "text",
					Index:         true,
					Store:         true,
					Highlightable: true,
				},
				"description": {
					Type:          "text",
					Index:         true,
					Store:         true,
					Highlightable: true,
				},
				"content": {
					Type:          "text",
					Index:         true,
					Store:         true,
					Highlightable: true,
				},
			},
		},
		//Settings: &client.MetaIndexSettings{
		//	Analysis: &client.MetaIndexAnalysis{
		//		Analyzer: &map[string]client.MetaAnalyzer{
		//			"default": {
		//				Type: &analyzer,
		//			},
		//		},
		//	},
		//},
	}
	_, r, err := s.apiClient.Index.Exists(s.ctx, s.indexName).Execute()
	if err != nil {
		log.Printf("Error when calling `Index.Exists``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}
	if r.StatusCode != 200 {
		_, r, err = s.apiClient.Index.Create(s.ctx).Data(data).Execute()
		if err != nil {
			log.Printf("Error when calling `Index.Create``: %v\n", err)
			log.Printf("Full HTTP response: %v\n", r)
			return err
		}
		if r.StatusCode != 200 {
			return errors.New("create index error")
		}
	}

	return nil
}

func (s *ZincSearchService) Create(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)
	_, r, err := s.apiClient.Document.IndexWithID(s.ctx, s.indexName, docId).Document(library.StructToMap(doc)).Execute()
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ZincSearchService) Update(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)

	_, r, err := s.apiClient.Document.Update(s.ctx, s.indexName, docId).Document(library.StructToMap(doc)).Execute()
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ZincSearchService) Delete(doc TinyArchive) error {
	id := doc.GetId()
	docId := strconv.FormatInt(id, 10)
	_, r, err := s.apiClient.Document.Delete(s.ctx, s.indexName, docId).Execute()
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ZincSearchService) Bulk(docs []TinyArchive) error {
	query := client.NewMetaJSONIngest()
	query.SetIndex(s.indexName)
	var data []map[string]interface{}
	for _, v := range docs {
		docId := v.GetId()
		item := library.StructToMap(v)
		// docId
		item["_id"] = strconv.FormatInt(docId, 10)

		data = append(data, item)
	}
	query.SetRecords(data)

	_, r, err := s.apiClient.Document.Bulkv2(s.ctx).Query(*query).Execute()
	if err != nil {
		log.Printf("Error when calling `Document.Index``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return err
	}

	return nil
}

func (s *ZincSearchService) Search(keyword string, moduleId uint, page int, pageSize int) ([]TinyArchive, int64, error) {
	if page < 1 {
		page = 1
	}

	queryQuery := *client.NewMetaQuery()
	query := *client.NewMetaZincQuery()
	subQueries := []client.MetaQuery{
		{Match: &map[string]client.MetaMatchQuery{"title": {Query: &keyword}}},
		{Match: &map[string]client.MetaMatchQuery{"keywords": {Query: &keyword}}},
		{Match: &map[string]client.MetaMatchQuery{"description": {Query: &keyword}}},
		{Match: &map[string]client.MetaMatchQuery{"content": {Query: &keyword}}},
	}

	minimumShouldMatch := float32(1)
	if moduleId > 0 {
		moduleIdStr := strconv.Itoa(int(moduleId))
		trimQuery := *client.NewMetaTermQuery()
		trimQuery.SetValue(moduleIdStr)
		subTrimQuery := *client.NewMetaQuery()
		subTrimQuery.SetTerm(map[string]client.MetaTermQuery{
			"type": trimQuery,
		})

		// {
		//  "bool": {
		//    "must": [
		//      {
		//        "bool": {
		//          "should": [
		//            { "match": { "title": "jone" } },
		//            { "match": { "description": "jone" } }
		//          ],
		//          "minimum_should_match": 1
		//        }
		//      },
		//      { "term": { "module_id": 2 } }
		//    ]
		//  }
		//}
		shouldBoolQuery := *client.NewMetaBoolQuery()
		shouldBoolQuery.SetShould(subQueries)
		shouldBoolQuery.MinimumShouldMatch = &minimumShouldMatch
		shouldQuery := *client.NewMetaQuery()
		shouldQuery.SetBool(shouldBoolQuery)

		boolQuery := *client.NewMetaBoolQuery()
		boolQuery.SetMust([]client.MetaQuery{shouldQuery, subTrimQuery})

		queryQuery.SetBool(boolQuery)
	} else {
		boolQuery := *client.NewMetaBoolQuery()
		boolQuery.SetShould(subQueries)
		boolQuery.MinimumShouldMatch = &minimumShouldMatch
		queryQuery.SetBool(boolQuery)
	}

	query.SetQuery(queryQuery)
	query.SetFrom(int32(pageSize * (page - 1)))
	query.SetSize(int32(pageSize))

	resp, r, err := s.apiClient.Search.Search(s.ctx, s.indexName).Query(query).Execute()
	if err != nil {
		log.Printf("Error when calling `SearchApi.Search``: %v\n", err)
		log.Printf("Full HTTP response: %v\n", r)
		return nil, 0, err
	}

	var docs = make([]TinyArchive, 0, pageSize)
	for _, hit := range resp.Hits.Hits {
		source := hit.GetSource()
		id, _ := strconv.ParseInt(hit.GetId(), 10, 64)
		doc := TinyArchive{}
		_ = library.MapToStruct(source, &doc)
		doc.Id, doc.Type = GetId(id)
		docs = append(docs, doc)
	}
	total := int64(*resp.Hits.Total.Value)

	return docs, total, nil
}

func (s *ZincSearchService) Close() {
	// nothing to do
}

func (s *ZincSearchService) Flush() {
	// nothing to do
}
