package fulltext

import (
	"errors"
	"strconv"
	"unicode/utf8"

	"github.com/fesiong/wukong/engine"
	"github.com/fesiong/wukong/types"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
)

// WukongService
// 悟空引擎，由于只有一个ID，因此，需要多 archive,category,tag 做ID区分
// ID 长度为18 位，
// 区分规则是：900000000000000000 = category，800000000000000000 = tag，其他 = archive
type WukongService struct {
	config   *config.PluginFulltextConfig
	searcher *engine.Engine
}

type DiyScoringCriteria struct {
	RankingScore float32
}

func (criteria DiyScoringCriteria) Score(doc types.IndexedDocument, fields interface{}) []float32 {
	// if doc.TokenProximity > MaxTokenProximity {
	// 	return []float32{}
	// }
	// 判断 RankingScore，如果评分低于 RankingScore 分，则排除
	if criteria.RankingScore > 0 && doc.BM25 < criteria.RankingScore {
		return []float32{}
	}

	return []float32{doc.BM25}
}

func NewWukongService(cfg *config.PluginFulltextConfig, indexName string) (Service, error) {
	s := &WukongService{
		config:   cfg,
		searcher: new(engine.Engine),
	}
	s.searcher.Init(types.EngineInitOptions{
		SegmenterDictionaries: config.ExecPath + "dictionary.txt",
	})

	return s, nil
}

func (s *WukongService) Index(body interface{}) error {
	// 内存缓存不需要实现
	return nil
}

func (s *WukongService) Create(doc TinyArchive) error {
	if s.searcher == nil {
		return nil
	}
	content := doc.Title
	if doc.Keywords != "" {
		content += " " + doc.Keywords
	}
	if doc.Description != "" {
		// 内容搜索的时候，需要去除html标签
		content += " " + library.StripTags(doc.Description)
	}
	if s.config.UseContent && doc.Content != "" {
		// 内容搜索的时候，需要去除html标签
		content += " " + library.StripTags(doc.Content)
	}
	// 对ID进行区分
	id := uint64(doc.GetId())
	s.searcher.IndexDocument(id, types.DocumentIndexData{
		Content: content,
		Labels:  []string{strconv.Itoa(int(doc.ModuleId))},
	}, false)

	return nil
}

func (s *WukongService) Update(doc TinyArchive) error {
	// 使用相同的处理方法
	return s.Create(doc)
}

func (s *WukongService) Delete(doc TinyArchive) error {
	id := doc.GetId()
	s.searcher.RemoveDocument(uint64(id), false)
	return nil
}

func (s *WukongService) Bulk(docs []TinyArchive) error {
	// 依然是逐个添加
	for _, v := range docs {
		_ = s.Create(v)
	}
	s.searcher.FlushIndex()

	return nil
}

func (s *WukongService) Search(keyword string, moduleId uint, page int, pageSize int) ([]TinyArchive, int64, error) {
	var err error
	if s.searcher == nil {
		err = errors.New("uninitialized")
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}

	var labels []string
	if moduleId > 0 {
		labels = append(labels, strconv.Itoa(int(moduleId)))
	}
	// 根据 ContainLength 截断搜索词，增强“至少包含x个字符”的效果
	text := keyword
	if s.config.ContainLength > 0 && utf8.RuneCountInString(keyword) > s.config.ContainLength {
		text = string([]rune(keyword)[:s.config.ContainLength])
	}
	output := s.searcher.Search(types.SearchRequest{
		Text:   text,
		Labels: labels,
		RankOptions: &types.RankOptions{
			ScoringCriteria: DiyScoringCriteria{RankingScore: float32(s.config.RankingScore)},
			OutputOffset:    pageSize * (page - 1),
			MaxOutputs:      pageSize,
		}})
	var ids []uint64
	for _, doc := range output.Docs {
		ids = append(ids, doc.DocId)
	}
	total := int64(output.NumDocs)
	var docs = make([]TinyArchive, 0, len(ids))
	if len(ids) > 0 {
		for _, id := range ids {
			doc := TinyArchive{}
			doc.Id, doc.Type = GetId(int64(id))
			docs = append(docs, doc)
		}
	}
	return docs, total, nil
}

func (s *WukongService) Close() {
	if s.searcher != nil {
		s.searcher.Close()
	}
}

func (s *WukongService) Flush() {
	if s.searcher != nil {
		s.searcher.FlushIndex()
	}
}
