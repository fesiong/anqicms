package provider

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/upyun/go-sdk/v3/upyun"
	"kandaoni.com/anqicms/config"
)

type BucketStorage struct {
	PublicPath          string
	config              *config.PluginStorageConfig
	tencentBucketClient *cos.Client
	aliyunBucketClient  *oss.Bucket
	qiniuBucketClient   *qbox.Mac
	upyunBucketClient   *upyun.UpYun
}

func (w *Website) GetBucket() (bucket *BucketStorage, err error) {
	bucket = &BucketStorage{
		PublicPath:          w.PublicPath,
		config:              &w.PluginStorage,
		tencentBucketClient: nil,
		aliyunBucketClient:  nil,
		qiniuBucketClient:   nil,
	}

	err = bucket.initBucket()

	return
}

func (bs *BucketStorage) initBucket() (err error) {
	if bs.config.StorageType == config.StorageTypeAliyun {
		err = bs.initAliyunBucket()
	} else if bs.config.StorageType == config.StorageTypeTencent {
		err = bs.initTencentBucket()
	} else if bs.config.StorageType == config.StorageTypeQiniu {
		err = bs.initQiniuBucket()
	} else if bs.config.StorageType == config.StorageTypeUpyun {
		err = bs.initUpyunBucket()
	} else {
		bs.config.StorageType = config.StorageTypeLocal
	}

	return err
}

func (bs *BucketStorage) UploadFile(location string, buff []byte) (string, error) {
	log.Println("存储到", bs.config.StorageType)
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

func (w *Website) InitBucket() {
	s, err := w.GetBucket()
	if err != nil {
		// 退回到local
		log.Println(err.Error())
		s.config.StorageType = config.StorageTypeLocal
	}
	w.Storage = s
}
