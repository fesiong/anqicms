package provider

import (
	"encoding/json"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Proxy 目前仅支持巨量IP

type ProxyIPs struct {
	cfg        *config.ProxyConfig
	IPs        []string
	index      int
	mu         sync.Mutex
	LastUpdate time.Time
}

func NewProxyIPs(cfg *config.ProxyConfig) *ProxyIPs {
	// 如果没填并发数量，则默认10并发
	if cfg.Concurrent <= 0 {
		cfg.Concurrent = 10
	}
	ips := &ProxyIPs{
		mu:  sync.Mutex{},
		IPs: make([]string, 0),
		cfg: cfg,
	}
	// 先获取1条
	ips.loadIPs()

	return ips
}

func (p *ProxyIPs) GetIP() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果IP列表为空，尝试加载新IP
	if len(p.IPs) == 0 {
		log.Println("empty proxy")
		// 控制并发
		sleepTime := time.Duration(1000/p.cfg.Concurrent) * time.Millisecond
		if time.Now().Add(-sleepTime).Before(p.LastUpdate) {
			time.Sleep(sleepTime)
		}
		p.loadIPs()
		p.LastUpdate = time.Now()
		if len(p.IPs) == 0 {
			return "" // 加载失败，返回空字符串
		}
		// 初次加载成功，直接返回第一个IP
		p.index = 0
		return p.IPs[0]
	}

	// 循环查找有效IP
	for attempts := 0; attempts < len(p.IPs); attempts++ {
		// 使用循环索引获取IP
		p.index = (p.index + 1) % len(p.IPs)
		ip := p.IPs[p.index]

		// 验证IP有效性
		ipUrl, err := url.Parse(ip)
		if err != nil {
			log.Printf("invalid IP format: %s, error: %v", ip, err)
			continue // 跳过格式不合法的IP
		}

		// 检查端口是否开放
		if library.ScanPort("tcp", ipUrl.Hostname(), ipUrl.Port()) {
			return ip // 有效IP直接返回
		}

		// 移除无效IP
		log.Printf("invalid IP: %s, removing it", ip)
		p.IPs = append(p.IPs[:p.index], p.IPs[p.index+1:]...)
		// 移除一个后，下标前移
		attempts--

		// 如果移除后IP列表为空，重新加载
		if len(p.IPs) == 0 {
			// 控制并发
			sleepTime := time.Duration(1000/p.cfg.Concurrent) * time.Millisecond
			if time.Now().Add(-sleepTime).Before(p.LastUpdate) {
				time.Sleep(sleepTime)
			}
			p.loadIPs()
			p.LastUpdate = time.Now()
			p.loadIPs()
			if len(p.IPs) == 0 {
				return "" // 无法加载更多IP，返回空字符串
			}
			// 重新加载后直接返回第一个IP
			p.index = 0
			return p.IPs[0]
		}
	}

	log.Println("no valid proxy IP found")
	return "" // 无有效IP可用
}

func (p *ProxyIPs) RemoveIP(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.IPs) == 0 {
		return
	}

	for i, v := range p.IPs {
		if v == ip {
			p.IPs = append(p.IPs[0:i], p.IPs[i+1:]...)
			break
		}
	}
}

func (p *ProxyIPs) AddIPs(ips []string) {
	//p.mu.Lock()
	//defer p.mu.Unlock()
	// 需要去重
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		found := false
		for _, v := range p.IPs {
			if v == ip {
				found = true
				break
			}
		}
		if !found {
			p.IPs = append(p.IPs, ip)
		}
	}
}

func (p *ProxyIPs) loadIPs() {
	// 巨量IP的获取方式
	// p.config.Platform == "juliangip"
	resp, err := library.GetURLData(p.cfg.ApiUrl, "", 5)
	if err != nil {
		log.Println("load proxy error", err)
		return
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Count           int    `json:"count"`
			FilterCount     int    `json:"filter_count"`
			SurplusQuantity int    `json:"surplus_quantity"`
			ProxyList       string `json:"proxy_list"`
		} `json:"data"`
	}
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		// 是IP
		if strings.Contains(resp.Body, ":") {
			ips := strings.Split(resp.Body, "\n")
			for _, ip := range ips {
				ip = strings.TrimSpace(ip)
				if ip == "" {
					continue
				}
				tip := "http://" + ip
				p.AddIPs([]string{tip})
				if p.cfg.Expire > 0 {
					time.AfterFunc(time.Duration(p.cfg.Expire)*time.Second, func() {
						p.RemoveIP(tip)
					})
				}
			}
		}
		return
	}
	if result.Code != 200 {
		log.Println("load proxy error", result.Msg)
		return
	}

	ips := strings.Split(result.Data.ProxyList, "\n")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		tip := "http://" + ip
		p.AddIPs([]string{tip})
		if p.cfg.Expire > 0 {
			time.AfterFunc(time.Duration(p.cfg.Expire)*time.Second, func() {
				p.RemoveIP(tip)
			})
		}
	}

	p.AddIPs(ips)
}
