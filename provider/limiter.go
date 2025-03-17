package provider

import (
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"kandaoni.com/anqicms/config"
)

// traffic limiter

type VisitInfo struct {
	Buckets    [5]int // 5个时间桶，每个桶存储一分钟的请求次数
	LastVisit  int64  // 上次访问的时间，用于判断时间桶的有效性
	CurrentIdx int    // 当前使用的桶的索引
	TotalCount int    // 窗口内的总请求次数
}

type BlockIP struct {
	IP        string `json:"ip"`
	BlockTime int64  `json:"block_time"`
}

type Limiter struct {
	mu            sync.Mutex
	ipVisits      map[string]*VisitInfo
	blockedIPs    map[string]time.Time
	blackIPs      []string // 黑名单IP，支持IP段
	whiteIPs      []string // 白名单IP，支持IP段
	blockAgents   []string
	allowPrefixes []string
	isAllowSpider bool
	banEmptyAgent bool
	banEmptyRefer bool
	MaxTime       time.Duration
	MaxRequests   int
	BlockDuration time.Duration
}

func (w *Website) InitLimiter() {
	setting := w.GetLimiterSetting()
	if setting.Open == false {
		w.Limiter = nil
		return
	}
	if w.Limiter != nil {
		w.Limiter.UpdateLimiter(setting)
		return
	}
	w.Limiter = NewLimiter(setting)
}

func NewLimiter(setting *config.PluginLimiter) *Limiter {
	limiter := &Limiter{
		ipVisits:   make(map[string]*VisitInfo),
		blockedIPs: make(map[string]time.Time),
		MaxTime:    5 * time.Minute,
		mu:         sync.Mutex{},
	}

	limiter.UpdateLimiter(setting)
	// 启动定期清理计划
	limiter.startCleanupTask()

	return limiter
}

func (l *Limiter) UpdateLimiter(setting *config.PluginLimiter) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.whiteIPs = setting.WhiteIPs
	l.blackIPs = setting.BlackIPs
	l.isAllowSpider = setting.IsAllowSpider
	l.allowPrefixes = setting.AllowPrefixes
	l.blockAgents = setting.BlockAgents
	l.banEmptyRefer = setting.BanEmptyRefer
	l.banEmptyAgent = setting.BanEmptyAgent
	if setting.MaxRequests < 1 {
		setting.MaxRequests = 100
	}
	l.MaxRequests = setting.MaxRequests
	if setting.BlockHours < 1 {
		setting.BlockHours = 1
	}
	l.BlockDuration = time.Duration(setting.BlockHours) * time.Hour
}

func (l *Limiter) isPrivateIP(ip string) bool {
	// 判断是否是内网IP，或本地IP
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "localhost") {
		return true
	}

	return false
}

// RecordIPVisit 记录IP访问，并判断是否超出阈值，如果需要封禁，则返回false，IP正常则返回true
func (l *Limiter) RecordIPVisit(ip string) bool {
	// 如果是内网IP，则不限制
	if l.isPrivateIP(ip) {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().Unix()      // 当前的Unix时间（秒）
	currentMinute := now / 60 % 5 // 当前在5个桶中的索引

	// 检查是否已有该IP的访问记录
	visitInfo, exists := l.ipVisits[ip]
	if !exists {
		visitInfo = &VisitInfo{
			Buckets:    [5]int{},
			LastVisit:  now,
			CurrentIdx: int(currentMinute),
		}
		l.ipVisits[ip] = visitInfo
	}

	// 计算当前时间和最后一次访问的时间差，更新桶的状态
	elapsedMinutes := int(now/60 - visitInfo.LastVisit/60)

	// 如果时间超过了窗口大小，重置所有桶
	if elapsedMinutes >= 5 {
		visitInfo.Buckets = [5]int{}
		visitInfo.TotalCount = 0
	} else {
		// 依次清理过期的桶
		for i := 1; i <= elapsedMinutes; i++ {
			idx := (visitInfo.CurrentIdx + i) % 5
			visitInfo.TotalCount -= visitInfo.Buckets[idx]
			visitInfo.Buckets[idx] = 0
		}
	}

	// 更新当前桶的索引和计数
	visitInfo.CurrentIdx = int(currentMinute)
	visitInfo.Buckets[visitInfo.CurrentIdx]++
	visitInfo.TotalCount++

	// 更新最后访问时间
	visitInfo.LastVisit = now

	// 检查是否超过最大请求次数
	if visitInfo.TotalCount > l.MaxRequests {
		return false // 超过最大请求次数，应该封禁
	}

	return true
}

// BlockIP 封禁IP
func (l *Limiter) BlockIP(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.blockedIPs[ip] = time.Now().Add(l.BlockDuration) // 封禁1小时
}

// IsIPBlocked 检查IP是否被封禁
func (l *Limiter) IsIPBlocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.blackIPs) > 0 {
		parsedIP := net.ParseIP(ip)
		// 检查是否在黑名单中，黑名单支持IP段
		for _, blackIP := range l.blackIPs {
			if strings.Contains(blackIP, "/") {
				// 检查IP段
				_, ipNet, err := net.ParseCIDR(blackIP)
				if err != nil {
					// 解析错误，忽略这条记录
					continue
				}
				// 判断IP是否在IP段中
				if ipNet.Contains(parsedIP) {
					// 在IP段内
					return true
				}
			} else if blackIP == ip {
				return true
			}
		}
	}

	unblockTime, blocked := l.blockedIPs[ip]
	if !blocked {
		return false
	}

	// 检查封禁时间是否已过
	if time.Now().After(unblockTime) {
		delete(l.blockedIPs, ip) // 解禁
		return false
	}

	return true
}

func (l *Limiter) IsWhiteIp(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.whiteIPs) == 0 {
		return false
	}
	parsedIP := net.ParseIP(ip)
	// 检查是否在白名单中，白名单支持IP段
	for _, whiteIP := range l.whiteIPs {
		if strings.Contains(whiteIP, "/") {
			// 检查IP段
			_, ipNet, err := net.ParseCIDR(whiteIP)
			if err != nil {
				// 解析错误，忽略这条记录
				continue
			}
			// 判断IP是否在IP段中
			if ipNet.Contains(parsedIP) {
				// 在IP段内
				return true
			}
		} else if whiteIP == ip {
			return true
		}
	}

	return false
}

func (l *Limiter) IsAllowSpider() bool {
	return l.isAllowSpider
}

func (l *Limiter) IsBanEmptyAgent() bool {
	return l.banEmptyAgent
}

func (l *Limiter) IsBanEmptyRefer() bool {
	return l.banEmptyRefer
}

func (l *Limiter) GetBlockIPs() []BlockIP {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	var ips = make([]BlockIP, 0, len(l.blockedIPs))
	for ip, t := range l.blockedIPs {
		ips = append(ips, BlockIP{
			IP:        ip,
			BlockTime: t.Unix(),
		})
	}
	sort.Slice(ips, func(i, j int) bool {
		return ips[i].BlockTime > ips[j].BlockTime
	})
	return ips
}

// RemoveBlockedIP 解禁某一个IP，用于管理员手动解禁
func (l *Limiter) RemoveBlockedIP(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.blockedIPs, ip)
	delete(l.ipVisits, ip)
}

func (l *Limiter) cleanupExpiredRecords() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().Unix()
	// 创建临时map记录需要保留的条目
	keepIPs := make(map[string]struct{})
	for ip, info := range l.ipVisits {
		if now-info.LastVisit <= int64(l.MaxTime.Seconds()) {
			keepIPs[ip] = struct{}{}
		}
	}
	// 重建ipVisits
	newIPVisits := make(map[string]*VisitInfo)
	for ip := range keepIPs {
		newIPVisits[ip] = l.ipVisits[ip]
	}
	l.ipVisits = newIPVisits

	// 重建blockedIPs
	newBlockedIPs := make(map[string]time.Time)
	for ip, unblockTime := range l.blockedIPs {
		if time.Now().Before(unblockTime) {
			newBlockedIPs[ip] = unblockTime
		}
	}
	l.blockedIPs = newBlockedIPs
}

func (l *Limiter) startCleanupTask() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			l.cleanupExpiredRecords()
		}
	}()
}
