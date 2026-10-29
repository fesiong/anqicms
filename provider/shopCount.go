package provider

import (
	"encoding/json"
	"fmt"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"os"
	"strings"
	"sync"
	"time"
)

// shop count log
type ShopCountLog struct {
	initial  bool
	rwMu     *sync.RWMutex
	filePath string
	Path     string
	Count    *model.ShopCount
	cap      int // 日志保留天数
	NewData  bool
	lastTime time.Time
	ticker   *time.Ticker // 定时器
	quit     chan bool    // 退出信号
}

func NewShopCountCache(path string) (*ShopCountLog, error) {
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		//新建
		_ = os.MkdirAll(path, 0755)
	}

	// 读取今日日期
	today := time.Now()
	sl := &ShopCountLog{
		rwMu:  &sync.RWMutex{},
		Path:  path,
		cap:   30,
		Count: &model.ShopCount{},
	}
	err = sl.newLog(today)
	if nil != err {
		sl.initial = false
		//打开失败，不做记录
		return sl, err
	}
	sl.initial = true

	// 启动定时器，每10秒刷新一次缓冲区
	sl.ticker = time.NewTicker(10 * time.Second)
	go sl.flushBuffer()

	return sl, nil
}

// newLog 新建日志文件
func (s *ShopCountLog) newLog(lt time.Time) error {
	if s == nil {
		return nil
	}
	filePath := fmt.Sprintf("%sshop-%s.log", s.Path, lt.Format("20060102"))
	var shopCount model.ShopCount

	buf, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(buf, &shopCount)
	}
	s.Count = &shopCount
	s.filePath = filePath
	s.lastTime = lt

	return nil
}

func (s *ShopCountLog) Write(req *request.LogStatisticRequest) error {
	if s == nil {
		return nil
	}
	if s.initial == false {
		return fmt.Errorf("log not initialized")
	}
	defer s.rwMu.Unlock()
	s.rwMu.Lock()
	// 添加到缓冲区
	if req.Type == request.LogTypeArticleView {
		s.Count.ArticleView++
	} else if req.Type == request.LogTypeProductView {
		s.Count.ProductView++
	} else if req.Type == request.LogTypeCouponGet {
		s.Count.CouponGet++
	} else if req.Type == request.LogTypeCouponUse {
		s.Count.CouponUse++
	} else if req.Type == request.LogTypeCart {
		s.Count.Cart++
	} else if req.Type == request.LogTypeCheckout {
		s.Count.Checkout++
	} else if req.Type == request.LogTypeConfirm {
		s.Count.Confirm++
	} else if req.Type == request.LogTypePayment {
		s.Count.Payment++
	} else if req.Type == request.LogTypeFirstOrder {
		s.Count.FirstOrder++
	} else if req.Type == request.LogTypeRepurchase {
		s.Count.Repurchase++
	} else if req.Type == request.LogTypeOrderCreate {
		s.Count.OrderCreate++
	} else if req.Type == request.LogTypePayCreate {
		s.Count.PayCreate++
	} else if req.Type == request.LogTypeUserCreate {
		s.Count.UserCreate++
	} else if req.Type == request.LogTypeUserLogin {
		s.Count.UserLogin++
	}
	s.NewData = true
	return nil
}

func (s *ShopCountLog) Close() {
	// 停止定时器
	if s.ticker != nil {
		s.ticker.Stop()
		s.quit <- true
	}
	// 刷新剩余数据
	s.doFlush()
}

// Clear 定期清理超过30天的日志文件, 每天执行一次
// flushBuffer 定时刷新缓冲区
func (s *ShopCountLog) flushBuffer() {
	for {
		select {
		case <-s.ticker.C:
			s.doFlush()
		case <-s.quit:
			return
		}
	}
}

// doFlush 执行实际的刷新操作
func (s *ShopCountLog) doFlush() {
	s.rwMu.Lock()
	defer s.rwMu.Unlock()

	if s.NewData == false {
		return
	}
	dayOfYear := time.Now().YearDay()
	if dayOfYear != s.lastTime.YearDay() {
		// 另一天了
		err := s.newLog(time.Now())
		if err != nil {
			return
		}
	}
	// 写入内容
	count, _ := json.Marshal(s.Count)
	_ = os.WriteFile(s.filePath, count, 0644)

	s.lastTime = time.Now()
	s.NewData = false
}
