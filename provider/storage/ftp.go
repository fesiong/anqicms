package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"kandaoni.com/anqicms/config"
)

type FtpStorage struct {
	cfg         *config.PluginStorageConfig
	client      *ftp.ServerConn
	tryTimes    int
	connectTime int64
	mu          sync.Mutex // 添加互斥锁以保护连接操作
}

func NewFtpStorage(cfg *config.PluginStorageConfig) (*FtpStorage, error) {
	s := &FtpStorage{
		cfg:         cfg,
		tryTimes:    0,
		connectTime: time.Now().Unix(),
	}
	err := s.init()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *FtpStorage) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已有可用连接，直接返回
	if s.client != nil {
		// 尝试发送NOOP命令测试连接
		if err := s.client.NoOp(); err == nil {
			return nil
		}
	}

	if s.tryTimes >= 3 {
		return errors.New("exceeded maximum retry attempts")
	}

	// 指数退避重试延迟
	if s.tryTimes > 0 {
		delay := time.Duration(math.Pow(2, float64(s.tryTimes))) * time.Second
		time.Sleep(delay)
	}

	var c *ftp.ServerConn
	var err error

	// 优先尝试TLS连接
	tlsConfig := &tls.Config{
		ServerName:         s.cfg.FTPHost,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}

	c, err = ftp.Dial(
		fmt.Sprintf("%s:%d", s.cfg.FTPHost, s.cfg.FTPPort),
		ftp.DialWithTimeout(10*time.Second),
		ftp.DialWithTLS(tlsConfig),
	)

	// 如果TLS连接失败，尝试普通连接
	if err != nil {
		log.Printf("TLS connection failed: %v, trying plain connection", err)
		c, err = ftp.Dial(
			fmt.Sprintf("%s:%d", s.cfg.FTPHost, s.cfg.FTPPort),
			ftp.DialWithTimeout(10*time.Second),
		)
		if err != nil {
			s.tryTimes++
			return fmt.Errorf("failed to establish connection: %v", err)
		}
	}

	if err = c.Login(s.cfg.FTPUsername, s.cfg.FTPPassword); err != nil {
		s.tryTimes++
		return fmt.Errorf("login failed: %v", err)
	}

	if err = c.ChangeDir(s.cfg.FTPWebroot); err != nil {
		s.tryTimes++
		return fmt.Errorf("change directory failed: %v", err)
	}

	// 重置重试计数和更新连接时间
	s.client = c
	s.tryTimes = 0
	s.connectTime = time.Now().Unix()

	return nil
}

func (s *FtpStorage) Put(ctx context.Context, key string, r io.Reader) error {
	// 确保连接可用
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %v", err)
	}

	// 创建目录结构
	remoteDir := path.Dir(key)
	dirs := strings.Split(remoteDir, "/")
	currentDir := s.cfg.FTPWebroot

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		currentDir += "/" + dir
		if err := s.createDirectory(currentDir); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", currentDir, err)
		}
	}

	// 上传文件
	remoteFile := s.cfg.FTPWebroot + "/" + strings.TrimLeft(key, "/")
	if err := s.uploadFile(remoteFile, r); err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	return nil
}

// 辅助方法：确保连接可用
func (s *FtpStorage) ensureConnection() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return s.init()
	}

	if err := s.client.NoOp(); err != nil {
		return s.init()
	}
	return nil
}

// 辅助方法：创建目录
func (s *FtpStorage) createDirectory(dir string) error {
	if err := s.client.ChangeDir(dir); err != nil {
		if err := s.client.MakeDir(dir); err != nil {
			if err := s.init(); err != nil {
				return err
			}
			return s.client.MakeDir(dir)
		}
	}
	return nil
}

// 辅助方法：上传文件
func (s *FtpStorage) uploadFile(remoteFile string, r io.Reader) error {
	if err := s.client.Stor(remoteFile, r); err != nil {
		if err := s.init(); err != nil {
			return err
		}
		return s.client.Stor(remoteFile, r)
	}
	return nil
}

func (s *FtpStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("connection check failed: %v", err)
	}

	remoteFile := s.cfg.FTPWebroot + "/" + strings.TrimLeft(key, "/")
	resp, err := s.client.Retr(remoteFile)
	if err != nil {
		// 如果是连接错误，尝试重新连接并重试
		if err := s.init(); err != nil {
			return nil, err
		}
		resp, err = s.client.Retr(remoteFile)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve file: %v", err)
		}
	}
	return resp, nil
}

func (s *FtpStorage) Delete(ctx context.Context, key string) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %v", err)
	}

	remoteFile := s.cfg.FTPWebroot + "/" + strings.TrimLeft(key, "/")
	if err := s.client.Delete(remoteFile); err != nil {
		// 如果是连接错误，尝试重新连接并重试
		if err := s.init(); err != nil {
			return err
		}
		if err := s.client.Delete(remoteFile); err != nil {
			return fmt.Errorf("failed to delete file: %v", err)
		}
	}
	return nil
}

func (s *FtpStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.ensureConnection(); err != nil {
		return false, fmt.Errorf("connection check failed: %v", err)
	}

	remoteFile := s.cfg.FTPWebroot + "/" + strings.TrimLeft(key, "/")
	_, err := s.client.FileSize(remoteFile)
	if err == nil {
		return true, nil
	}

	// 如果是连接错误，尝试重新连接并重试
	if err := s.init(); err != nil {
		return false, err
	}
	_, err = s.client.FileSize(remoteFile)
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (s *FtpStorage) Move(ctx context.Context, src, dest string) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %v", err)
	}
	realSrc := s.cfg.FTPWebroot + "/" + strings.TrimLeft(src, "/")
	realDest := s.cfg.FTPWebroot + "/" + strings.TrimLeft(dest, "/")
	return s.client.Rename(realSrc, realDest)
}
