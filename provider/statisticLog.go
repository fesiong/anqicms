package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jinzhu/now"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/model"
)

type Statistic struct {
	Id          uint   `json:"id"`
	CreatedTime int64  `json:"created_time"`
	Spider      string `json:"spider"`
	Host        string `json:"host"`
	Url         string `json:"url"`
	Ip          string `json:"ip"`
	Device      string `json:"device"`
	HttpCode    int    `json:"http_code"`
	UserAgent   string `json:"user_agent"`
}

type StatisticLog struct {
	initial  bool
	file     *os.File
	rwMu     *sync.RWMutex
	Path     string
	cap      int // 日志保留天数
	lastTime time.Time
	totals   map[string]int
	buffer   []*Statistic // 缓冲区
	ticker   *time.Ticker // 定时器
	quit     chan bool    // 退出信号
}

func NewStatisticLog(path string) (*StatisticLog, error) {
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
	sl := &StatisticLog{
		rwMu:   &sync.RWMutex{},
		Path:   path,
		totals: make(map[string]int),
		cap:    30,
		buffer: make([]*Statistic, 0),
		quit:   make(chan bool),
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
func (s *StatisticLog) newLog(lt time.Time) error {
	filePath := fmt.Sprintf("%s%s.log", s.Path, lt.Format("20060102"))
	logFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	s.file = logFile
	s.lastTime = lt

	return nil
}

func (s *StatisticLog) Close() {
	// 停止定时器
	if s.ticker != nil {
		s.ticker.Stop()
		s.quit <- true
	}
	// 刷新剩余数据
	s.doFlush()
	_ = s.file.Close()
}

func (s *StatisticLog) Write(data *Statistic) error {
	if s.initial == false {
		return fmt.Errorf("log not initialized")
	}
	if data.CreatedTime == 0 {
		data.CreatedTime = time.Now().Unix()
	}

	defer s.rwMu.Unlock()
	s.rwMu.Lock()
	// 添加到缓冲区
	s.buffer = append(s.buffer, data)
	return nil
}

func (s *StatisticLog) Read(fileName string, offset, limit int) ([]*Statistic, int64) {
	if s.initial == false {
		return nil, 0
	}
	defer s.rwMu.Unlock()
	s.rwMu.Lock()

	if fileName == "" {
		fileName = time.Now().Format("20060102")
	}
	filePath := fmt.Sprintf("%s%s.log", s.Path, fileName)
	logFile, err := os.Open(filePath)
	if nil != err {
		//打开失败，不做记录
		return nil, 0
	}
	defer logFile.Close()
	var fileSize int64
	// 获取文件大小
	if fileInfo, err := logFile.Stat(); err == nil {
		fileSize = fileInfo.Size()
	} else {
		return nil, 0
	}

	reader := bufio.NewReader(logFile)
	// 每次读取8K
	buffer := make([]byte, 8192)
	lineBuffer := ""

	// 按limit行读取
	var lines = make([]string, 0, limit)
	var curPos = fileSize
	var curLine = 0

	// 倒序读取文件内容
	for curPos > 0 {
		// 定位到需要读取的位置
		bytesToRead := int64(len(buffer))
		if curPos-bytesToRead < 0 {
			bytesToRead = curPos
		}

		// 调整文件指针到合适位置
		curPos -= bytesToRead
		logFile.Seek(curPos, io.SeekStart)

		// 读取文件数据到缓冲区
		n, err := reader.Read(buffer[:bytesToRead])
		if err != nil && err != io.EOF {
			return nil, 0
		}

		// 将读到的数据加入行缓冲
		lineBuffer = string(buffer[:n]) + lineBuffer

		// 处理行
		for {
			newLineIdx := len(lineBuffer) - 1
			for newLineIdx >= 0 && lineBuffer[newLineIdx] != '\n' {
				newLineIdx--
			}

			// 找到完整的行
			if newLineIdx == -1 {
				break
			}

			line := lineBuffer[newLineIdx+1:]
			lineBuffer = lineBuffer[:newLineIdx]

			if line != "" {
				curLine++
				if curLine <= offset {
					continue
				}
				lines = append(lines, line)

				// 如果已经获取到需要的行数，跳出循环
				if len(lines) >= limit {
					break
				}
			}
		}

		// 处理剩余内容
		if curPos == 0 && lineBuffer != "" && len(lines) < limit {
			lines = append(lines, lineBuffer)
		}
		// 如果已经获取到需要的行数，跳出循环
		if len(lines) >= limit {
			break
		}
	}

	var result = make([]*Statistic, 0, len(lines))
	for _, line := range lines {
		var data Statistic
		err = json.Unmarshal([]byte(line), &data)
		if err != nil {
			continue
		}
		result = append(result, &data)
	}
	// 如果文件计数缓存，则重新计数
	total, ok := s.totals[fileName]
	if !ok {
		logFile.Seek(0, io.SeekStart)
		sc := bufio.NewScanner(reader)
		for sc.Scan() {
			total++
		}
		// 如果是当天的文件，则不缓存
		if fileName != time.Now().Format("20060102") {
			s.totals[fileName] = total
		}
	}

	return result, int64(total)
}

// Calc 每10分钟进行一次统计，只统计 200 状态的日志
func (s *StatisticLog) Calc(db *gorm.DB) {
	if s.initial == false {
		return
	}
	// 读取数据库最后一条记录，如果最后一条记录不是当天的，则重新统计
	today := now.BeginningOfDay()
	startTime := today
	var lastStatistic model.StatisticLog
	err := db.Model(&model.StatisticLog{}).Order("id DESC").Take(&lastStatistic).Error
	if err == nil {
		// 存在最后一条
		lastTime := time.Unix(lastStatistic.CreatedTime, 0)
		if lastTime.YearDay() != today.YearDay() {
			// 重新统计
			logData, err := s.CalcLog(lastTime)
			if err == nil {
				// 记录存在，更新数据库
				lastStatistic.SpiderCount = logData.SpiderCount
				lastStatistic.VisitCount = logData.VisitCount
				db.Save(&lastStatistic)
			}
		}
		startTime = lastTime.AddDate(0, 0, 1)
	} else {
		// 定义在30天前
		startTime = today.AddDate(0, 0, -s.cap)
	}
	// 对历史的进行统计
	for {
		if startTime.YearDay() >= today.YearDay() {
			break
		}
		logData, err := s.CalcLog(startTime)
		if err == nil {
			// 记录存在，写入数据库
			db.Model(&model.StatisticLog{}).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "created_time"}},
				UpdateAll: true,
			}).Where("`created_time` = ?", startTime.Unix()).Create(&logData)
		}
		startTime = startTime.AddDate(0, 0, 1)
	}
	// 对今天的进行统计
	logData, err := s.CalcLog(today)
	if err == nil {
		// 记录存在，写入数据库
		db.Model(&model.StatisticLog{}).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "created_time"}},
			UpdateAll: true,
		}).Where("`created_time` = ?", today.Unix()).Create(&logData)
	}
}

func (s *StatisticLog) CalcLog(lt time.Time) (*model.StatisticLog, error) {
	if s.initial == false {
		return nil, fmt.Errorf("log not initialized")
	}
	filePath := fmt.Sprintf("%s%s.log", s.Path, lt.Format("20060102"))
	logFile, err := os.Open(filePath)
	if nil != err {
		//打开失败，不做记录
		return nil, err
	}
	defer logFile.Close()
	// 每次读取10MB，减少IO
	buff := make([]byte, 1024*1024*10)
	var lineBuffer []byte
	reader := bufio.NewReader(logFile)
	var statistic = model.StatisticLog{
		CreatedTime: lt.Unix(),
		SpiderCount: model.SpiderCount{},
		VisitCount:  model.VisitCount{},
	}
	// ip 需要去重
	var ipMap = map[string]struct{}{}

	for {
		n, err := reader.Read(buff)
		if err != nil {
			if err == io.EOF {
				break
			}
			// 也结束循环
			break
		}
		lineBuffer = append(lineBuffer, buff[:n]...)
		// 逐行处理
		for {
			lnIdx := bytes.IndexByte(lineBuffer, '\n')
			if lnIdx == -1 {
				break
			}
			line := lineBuffer[:lnIdx]
			lineBuffer = lineBuffer[lnIdx+1:]

			if len(line) > 0 {
				var data Statistic
				err = json.Unmarshal(line, &data)
				if err != nil {
					continue
				}
				// 统计数据
				if data.Spider != "" {
					statistic.SpiderCount[data.Spider]++
				} else if data.HttpCode >= 200 && data.HttpCode < 300 {
					statistic.VisitCount.PVCount++
					ipMap[data.Ip] = struct{}{}
				}
			}
		}
		// 处理剩余内容
		if len(lineBuffer) > 0 {
			var data Statistic
			err = json.Unmarshal(lineBuffer, &data)
			if err != nil {
				continue
			}
			// 统计数据
			if data.Spider != "" {
				statistic.SpiderCount[data.Spider]++
			} else if data.HttpCode >= 200 && data.HttpCode < 300 {
				statistic.VisitCount.PVCount++
				ipMap[data.Ip] = struct{}{}
			}
		}
	}
	// 统计ip
	statistic.VisitCount.IPCount = len(ipMap)

	return &statistic, nil
}

func (s *StatisticLog) GetLogDates() []string {
	if s.initial == false {
		return nil
	}
	var paths = make([]string, 0, 31)

	fileInfos, _ := os.ReadDir(s.Path)
	for _, fileInfo := range fileInfos {
		fileName := fileInfo.Name()
		if !strings.HasSuffix(fileName, ".log") {
			continue
		}
		fileName = strings.TrimSuffix(fileName, ".log")
		paths = append(paths, fileName)
	}
	// 对 paths 进行倒叙排序
	sort.Slice(paths, func(i, j int) bool {
		return paths[i] > paths[j]
	})

	return paths
}

// Clear 定期清理超过30天的日志文件, 每天执行一次
// flushBuffer 定时刷新缓冲区
func (s *StatisticLog) flushBuffer() {
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
func (s *StatisticLog) doFlush() {
	s.rwMu.Lock()
	defer s.rwMu.Unlock()

	if len(s.buffer) == 0 {
		return
	}

	// 按日期分组
	dataMap := make(map[int][]*Statistic)
	for _, data := range s.buffer {
		createTime := time.Unix(data.CreatedTime, 0)
		dayOfYear := createTime.YearDay()
		dataMap[dayOfYear] = append(dataMap[dayOfYear], data)
	}

	// 按日期写入文件
	for _, dataList := range dataMap {
		if len(dataList) == 0 {
			continue
		}
		// 使用第一条数据的时间作为基准
		createTime := time.Unix(dataList[0].CreatedTime, 0)
		if createTime.YearDay() != s.lastTime.YearDay() {
			_ = s.file.Close()
			err := s.newLog(createTime)
			if err != nil {
				continue
			}
		}
		s.lastTime = createTime

		// 批量写入
		var bufs [][]byte
		for _, data := range dataList {
			buf, err := json.Marshal(data)
			if err != nil {
				continue
			}
			bufs = append(bufs, buf)
		}
		joindBuf := bytes.Join(bufs, []byte("\n"))
		joindBuf = append(joindBuf, []byte("\n")...)
		_, _ = s.file.Write(joindBuf)
	}

	// 清空缓冲区
	s.buffer = make([]*Statistic, 0)
}

func (s *StatisticLog) Clear(force bool) {
	if s.initial == false {
		return
	}
	// 需要保留的日志
	keepFiles := map[string]struct{}{}
	if !force {
		for i := 0; i < s.cap; i++ {
			curTime := s.lastTime.AddDate(0, 0, -i)
			keepFiles[curTime.Format("20060102")] = struct{}{}
		}
	}
	// 遍历日志文件夹，删除不在 keepFiles 列表的文件
	fileInfos, _ := os.ReadDir(s.Path)
	for _, fileInfo := range fileInfos {
		fileName := fileInfo.Name()
		if !strings.HasSuffix(fileName, ".log") {
			continue
		}
		fileName = strings.TrimSuffix(fileName, ".log")
		if _, ok := keepFiles[fileName]; !ok {
			_ = os.Remove(fmt.Sprintf("%s%s", s.Path, fileInfo.Name()))
		}
	}
}

func (w *Website) InitStatistic() {
	var err error
	w.StatisticLog, err = NewStatisticLog(w.RootPath + "data/statistic/")
	if err != nil {
		fmt.Println("InitStatisticLog error:", err)
		return
	}
	// 先从旧站点迁移数据
	// 处理 statistic
	if w.DB.Migrator().HasTable(&Statistic{}) {
		lastId := uint(0)
		for {
			var stats []Statistic
			// 一次写入5000条
			w.DB.Model(&Statistic{}).Where("id > ?", lastId).Order("id ASC").Limit(5000).Scan(&stats)
			if len(stats) == 0 {
				break
			}
			// 按天写入
			for _, stat := range stats {
				w.StatisticLog.Write(&stat)
			}
			lastId = stats[len(stats)-1].Id
		}

		w.DB.Migrator().DropTable(&Statistic{})
	}

}
