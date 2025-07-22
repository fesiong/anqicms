package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/melbahja/goph"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"kandaoni.com/anqicms/config"
)

type SSHStorage struct {
	cfg         *config.PluginStorageConfig
	client      *sftp.Client
	dataPath    string
	tryTimes    int
	connectTime int64
	mu          sync.Mutex // 添加互斥锁以保护连接操作
}

func NewSSHStorage(cfg *config.PluginStorageConfig, dataPath string) (*SSHStorage, error) {
	s := &SSHStorage{
		cfg:         cfg,
		tryTimes:    0,
		connectTime: time.Now().Unix(),
		dataPath:    dataPath,
	}
	err := s.init()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SSHStorage) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已有可用连接，直接返回
	if s.client != nil {
		// 尝试发送空操作测试连接
		if _, err := s.client.Stat("."); err == nil {
			return nil
		}
		// 关闭失效的连接
		s.client.Close()
	}

	if s.tryTimes >= 3 {
		return errors.New("exceeded maximum retry attempts")
	}

	// 指数退避重试延迟
	if s.tryTimes > 0 {
		delay := time.Duration(math.Pow(2, float64(s.tryTimes))) * time.Second
		time.Sleep(delay)
	}

	var auth goph.Auth
	var err error
	s.tryTimes++

	if s.cfg.SSHPrivateKey != "" {
		//尝试使用私钥连接
		filePath := fmt.Sprintf(s.dataPath + "cert/" + s.cfg.SSHPrivateKey)
		auth, err = goph.Key(filePath, s.cfg.SSHPassword)
		if err != nil {
			return err
		}
	} else {
		auth = goph.Password(s.cfg.SSHPassword)
	}
	client, err := goph.NewConn(&goph.Config{
		User:     s.cfg.SSHUsername,
		Addr:     s.cfg.SSHHost,
		Port:     uint(s.cfg.SSHPort),
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return err
	}
	sshClient, err := client.NewSftp()
	if err != nil {
		return err
	}
	// 验证权限
	tmpFile := s.cfg.SSHWebroot + "/tmp.tmp"
	file, err := sshClient.Create(tmpFile)
	if err != nil {
		return err
	}
	file.Close()
	sshClient.Remove(tmpFile)

	s.client = sshClient
	s.tryTimes = 0
	s.connectTime = time.Now().Unix()

	return nil
}

// 辅助方法：确保连接可用
func (s *SSHStorage) ensureConnection() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return s.init()
	}

	// 测试连接是否有效
	if _, err := s.client.Stat("."); err != nil {
		return s.init()
	}
	return nil
}

func (s *SSHStorage) Put(ctx context.Context, key string, r io.Reader) error {
	// 确保连接可用
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %v", err)
	}

	// 创建目录
	remoteDir := s.cfg.SSHWebroot + "/" + path.Dir(key)
	if err := s.client.MkdirAll(remoteDir); err != nil {
		log.Printf("Failed to create directory: %v, retrying", err)
		if err := s.init(); err != nil {
			return fmt.Errorf("failed to reinitialize connection: %v", err)
		}
		if err := s.client.MkdirAll(remoteDir); err != nil {
			return fmt.Errorf("failed to create directory after retry: %v", err)
		}
	}

	// 创建并写入文件
	remoteFile := s.cfg.SSHWebroot + "/" + strings.TrimLeft(key, "/")
	file, err := s.client.Create(remoteFile)
	if err != nil {
		log.Printf("Failed to create file: %v, retrying", err)
		if err := s.init(); err != nil {
			return fmt.Errorf("failed to reinitialize connection: %v", err)
		}
		file, err = s.client.Create(remoteFile)
		if err != nil {
			return fmt.Errorf("failed to create file after retry: %v", err)
		}
	}
	defer file.Close()

	// 正确处理io.Copy的错误
	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

func (s *SSHStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("connection check failed: %v", err)
	}

	remoteFile := s.cfg.SSHWebroot + "/" + strings.TrimLeft(key, "/")
	resp, err := s.client.Open(remoteFile)
	if err != nil {
		// 如果是连接错误，尝试重新连接并重试
		if err := s.init(); err != nil {
			return nil, fmt.Errorf("failed to reinitialize connection: %v", err)
		}
		resp, err = s.client.Open(remoteFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open file after retry: %v", err)
		}
	}
	return resp, nil
}

func (s *SSHStorage) Delete(ctx context.Context, key string) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %v", err)
	}

	remoteFile := s.cfg.SSHWebroot + "/" + strings.TrimLeft(key, "/")
	// 先检查文件是否存在
	if _, err := s.client.Stat(remoteFile); err != nil {
		return fmt.Errorf("file not found: %v", err)
	}

	// 尝试删除文件
	if err := s.client.Remove(remoteFile); err != nil {
		// 如果是连接错误，尝试重新连接并重试
		if err := s.init(); err != nil {
			return fmt.Errorf("failed to reinitialize connection: %v", err)
		}
		if err := s.client.Remove(remoteFile); err != nil {
			return fmt.Errorf("failed to delete file after retry: %v", err)
		}
	}
	return nil
}

func (s *SSHStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.ensureConnection(); err != nil {
		return false, fmt.Errorf("connection check failed: %v", err)
	}

	remoteFile := s.cfg.SSHWebroot + "/" + strings.TrimLeft(key, "/")
	_, err := s.client.Stat(remoteFile)
	if err == nil {
		return true, nil
	}

	// 如果是连接错误，尝试重新连接并重试
	if err := s.init(); err != nil {
		return false, fmt.Errorf("failed to reinitialize connection: %v", err)
	}
	_, err = s.client.Stat(remoteFile)
	if err == nil {
		return true, nil
	}

	// 文件不存在
	return false, nil
}

func (s *SSHStorage) Move(ctx context.Context, src, dest string) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %v", err)
	}
	realSrc := s.cfg.FTPWebroot + "/" + strings.TrimLeft(src, "/")
	realDest := s.cfg.FTPWebroot + "/" + strings.TrimLeft(dest, "/")
	return s.client.Rename(realSrc, realDest)
}
