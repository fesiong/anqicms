package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/jlaffaye/ftp"
	"github.com/melbahja/goph"
	"github.com/pkg/sftp"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/upyun/go-sdk/v3/upyun"
	"kandaoni.com/anqicms/config"
)

type BucketStorage struct {
	DataPath            string
	PublicPath          string
	w                   *Website
	config              *config.PluginStorageConfig
	tencentBucketClient *cos.Client
	aliyunBucketClient  *oss.Bucket
	qiniuBucketClient   *qbox.Mac
	upyunBucketClient   *upyun.UpYun
	ftpClient           *ftp.ServerConn
	sshClient           *sftp.Client
	tryTimes            int
	connectTime         int64
}

func (w *Website) GetBucket() (bucket *BucketStorage, err error) {
	bucket = &BucketStorage{
		DataPath:            w.DataPath,
		PublicPath:          w.PublicPath,
		w:                   w,
		config:              w.PluginStorage,
		tencentBucketClient: nil,
		aliyunBucketClient:  nil,
		qiniuBucketClient:   nil,
		tryTimes:            0,
	}

	err = bucket.initBucket()

	return
}

func (bs *BucketStorage) initBucket() (err error) {
	bs.tryTimes = 0
	if bs.config.StorageType == config.StorageTypeAliyun {
		err = bs.initAliyunBucket()
	} else if bs.config.StorageType == config.StorageTypeTencent {
		err = bs.initTencentBucket()
	} else if bs.config.StorageType == config.StorageTypeQiniu {
		err = bs.initQiniuBucket()
	} else if bs.config.StorageType == config.StorageTypeUpyun {
		err = bs.initUpyunBucket()
	} else if bs.config.StorageType == config.StorageTypeFTP {
		err = bs.initFTP()
	} else if bs.config.StorageType == config.StorageTypeSSH {
		err = bs.initSSH()
	} else {
		bs.config.StorageType = config.StorageTypeLocal
	}

	return err
}

func (bs *BucketStorage) UploadFile(location string, buff []byte) (string, error) {
	//log.Println("存储到", bs.config.StorageType)
	location = strings.TrimLeft(location, "/")
	if bs.config.KeepLocal || bs.config.StorageType == config.StorageTypeLocal {
		//将文件写入本地
		basePath := bs.PublicPath
		//先判断文件夹是否存在，不存在就先创建
		_, err := os.Stat(basePath + location)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Dir(basePath+location), os.ModePerm)
			if err != nil {
				return "", err
			}
		}
		err = os.WriteFile(basePath+location, buff, os.ModePerm)
		if err != nil {
			log.Println(err.Error())
			//无法创建
			return "", err
		}
		// 如果是本地存储，则
		// 上传到静态服务器
		_ = bs.w.SyncHtmlCacheToStorage(basePath+location, location)
	}
	if bs.config.StorageType == config.StorageTypeAliyun {
		if bs.aliyunBucketClient == nil {
			err := bs.initAliyunBucket()
			if err != nil {
				return "", err
			}
		}
		//不使用/开头
		err := bs.aliyunBucketClient.PutObject(location, bytes.NewReader(buff))
		if err != nil {
			return "", err
		}
	} else if bs.config.StorageType == config.StorageTypeTencent {
		if bs.tencentBucketClient == nil {
			err := bs.initTencentBucket()
			if err != nil {
				return "", err
			}
		}
		_, err := bs.tencentBucketClient.Object.Put(context.Background(), location, bytes.NewReader(buff), nil)
		if err != nil {
			return "", err
		}
	} else if bs.config.StorageType == config.StorageTypeQiniu {
		//log.Println("使用七牛云上传")
		if bs.qiniuBucketClient == nil {
			err := bs.initQiniuBucket()
			if err != nil {
				return "", err
			}
		}
		putPolicy := storage.PutPolicy{
			Scope: fmt.Sprintf("%s:%s", bs.config.QiniuBucket, location),
		}
		upToken := putPolicy.UploadToken(bs.qiniuBucketClient)

		cfg := storage.Config{}
		// 是否使用https域名
		//cfg.UseHTTPS = true
		// 上传是否使用CDN上传加速
		cfg.UseCdnDomains = false
		region, _ := storage.GetRegionByID(storage.RegionID(bs.config.QiniuRegion))
		cfg.Zone = &region
		formUploader := storage.NewFormUploader(&cfg)
		ret := storage.PutRet{}
		putExtra := storage.PutExtra{}
		err := formUploader.Put(context.Background(), &ret, upToken, location, bytes.NewReader(buff), -1, &putExtra)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
	} else if bs.config.StorageType == config.StorageTypeUpyun {
		if bs.upyunBucketClient == nil {
			err := bs.initUpyunBucket()
			if err != nil {
				return "", err
			}
		}
		err := bs.upyunBucketClient.Put(&upyun.PutObjectConfig{
			Path:   location,
			Reader: bytes.NewReader(buff),
		})

		if err != nil {
			fmt.Println(err)
			return "", err
		}
	} else if bs.config.StorageType == config.StorageTypeFTP {
		if bs.ftpClient == nil {
			err := bs.initFTP()
			if err != nil {
				return "", err
			}
		}
		// 尝试创建目录
		remoteDir := path.Dir(location)
		dirs := strings.Split(remoteDir, "/")
		addonDir := bs.config.FTPWebroot
		for _, v := range dirs {
			addonDir += "/" + v
			//尝试切换到目录，如果切换不成功，则尝试创建
			err := bs.ftpClient.ChangeDir(addonDir)
			if err != nil {
				//无法切换，则尝试创建
				err = bs.ftpClient.MakeDir(addonDir)
				if err != nil {
					err = bs.initSSH()
					if err != nil {
						return "", err
					}
					err = bs.ftpClient.MakeDir(addonDir)
					if err != nil {
						return "", err
					}
				}
			}
		}
		remoteFile := bs.config.FTPWebroot + "/" + strings.TrimLeft(location, "/")
		err := bs.ftpClient.Stor(remoteFile, bytes.NewReader(buff))
		if err != nil {
			log.Println("尝试重连：", err)
			err = bs.initFTP()
			if err == nil {
				return bs.UploadFile(location, buff)
			}
			return "", errors.New(bs.w.Tr("UploadFileFailedLog", remoteFile, err.Error()))
		}
	} else if bs.config.StorageType == config.StorageTypeSSH {
		if bs.sshClient == nil {
			err := bs.initSSH()
			if err != nil {
				return "", err
			}
		}
		// 尝试创建目录
		remoteDir := bs.config.SSHWebroot + "/" + path.Dir(location)
		err := bs.sshClient.MkdirAll(remoteDir)
		if err != nil {
			log.Println(err)
			err = bs.initSSH()
			if err != nil {
				return "", err
			}
			err = bs.sshClient.MkdirAll(remoteDir)
			if err != nil {
				return "", err
			}
		}
		remoteFile := bs.config.SSHWebroot + "/" + strings.TrimLeft(location, "/")
		file, err := bs.sshClient.Create(remoteFile)
		if err != nil {
			log.Println("尝试重连：", err.Error())
			err = bs.initSSH()
			if err == nil {
				return bs.UploadFile(location, buff)
			}
			return "", errors.New(bs.w.Tr("UploadFileFailedLog", remoteFile, err.Error()))
		}
		file.Write(buff)
		file.Close()
	}

	return location, nil
}

func (bs *BucketStorage) initTencentBucket() error {
	u, _ := url.Parse(bs.config.TencentBucketUrl)
	b := &cos.BaseURL{BucketURL: u}
	// Permanent key
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  bs.config.TencentSecretId,
			SecretKey: bs.config.TencentSecretKey,
		},
	})

	bs.tencentBucketClient = client

	return nil
}

func (bs *BucketStorage) initAliyunBucket() error {
	client, err := oss.New(bs.config.AliyunEndpoint, bs.config.AliyunAccessKeyId, bs.config.AliyunAccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(bs.config.AliyunBucketName)
	if err != nil {
		return err
	}

	bs.aliyunBucketClient = bucket

	return nil
}

func (bs *BucketStorage) initQiniuBucket() error {
	mac := qbox.NewMac(bs.config.QiniuAccessKey, bs.config.QiniuSecretKey)

	bs.qiniuBucketClient = mac

	return nil
}

func (bs *BucketStorage) initUpyunBucket() error {
	up := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   bs.config.UpyunBucket,
		Operator: bs.config.UpyunOperator,
		Password: bs.config.UpyunPassword,
	})

	bs.upyunBucketClient = up

	return nil
}

func (bs *BucketStorage) initFTP() error {
	if bs.tryTimes >= 3 || time.Now().Unix() < bs.connectTime+120 {
		return errors.New(bs.w.Tr("RetryMoreThan3Times"))
	}
	var c *ftp.ServerConn
	var err error
	bs.tryTimes++
	tlsConfig := &tls.Config{
		ServerName:         bs.config.FTPHost,
		InsecureSkipVerify: true,
	}
	c, err = ftp.Dial(
		fmt.Sprintf("%s:%d", bs.config.FTPHost, bs.config.FTPPort),
		ftp.DialWithTimeout(5*time.Second),
		//ftp.DialWithDebugOutput(os.Stdout),
		//ftp.DialWithExplicitTLS(tlsConfig),
		ftp.DialWithTLS(tlsConfig),
	)
	if err != nil {
		log.Println("尝试TLS 链接失败，重试普通链接：", err.Error())
		//再尝试使用普通链接
		c, err = ftp.Dial(
			fmt.Sprintf("%s:%d", bs.config.FTPHost, bs.config.FTPPort),
			ftp.DialWithTimeout(5*time.Second),
			//ftp.DialWithDebugOutput(os.Stdout),
		)
		if err != nil {
			return err
		}
	}

	err = c.Login(bs.config.FTPUsername, bs.config.FTPPassword)
	if err != nil {
		return err
	}

	err = c.ChangeDir(bs.config.FTPWebroot)
	if err != nil {
		return err
	}
	bs.ftpClient = c
	bs.tryTimes = 0
	bs.connectTime = time.Now().Unix()

	return nil
}

func (bs *BucketStorage) initSSH() error {
	if bs.tryTimes >= 3 || time.Now().Unix() < bs.connectTime+120 {
		return errors.New(bs.w.Tr("RetryMoreThan3Times"))
	}
	var auth goph.Auth
	var err error
	bs.tryTimes++

	if bs.config.SSHPrivateKey != "" {
		//尝试使用私钥连接
		filePath := fmt.Sprintf(bs.DataPath + "cert/" + bs.config.SSHPrivateKey)
		auth, err = goph.Key(filePath, bs.config.SSHPassword)
		if err != nil {
			return err
		}
	} else {
		auth = goph.Password(bs.config.SSHPassword)
	}
	client, err := goph.NewConn(&goph.Config{
		User:     bs.config.SSHUsername,
		Addr:     bs.config.SSHHost,
		Port:     uint(bs.config.SSHPort),
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
	tmpFile := bs.config.SSHWebroot + "/tmp.tmp"
	file, err := sshClient.Create(tmpFile)
	if err != nil {
		return err
	}
	file.Close()
	sshClient.Remove(tmpFile)

	bs.sshClient = sshClient
	bs.tryTimes = 0
	bs.connectTime = time.Now().Unix()

	return nil
}

func (w *Website) InitBucket() {
	s, err := w.GetBucket()
	if err != nil {
		// 退回到local
		log.Println(err.Error())
		s.config.StorageType = config.StorageTypeLocal
	}
	w.Storage = s
}
