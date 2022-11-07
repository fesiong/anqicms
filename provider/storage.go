package provider

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
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

var Storage *BucketStorage

type BucketStorage struct {
	storageType         string
	tencentBucketClient *cos.Client
	aliyunBucketClient  *oss.Bucket
	qiniuBucketClient   *qbox.Mac
	upyunBucketClient   *upyun.UpYun
}

func GetBucket() (bucket *BucketStorage, err error) {
	bucket = &BucketStorage{
		storageType:         config.JsonData.PluginStorage.StorageType,
		tencentBucketClient: nil,
		aliyunBucketClient:  nil,
		qiniuBucketClient:   nil,
	}

	err = bucket.InitBucket()

	return
}

func (bs *BucketStorage) InitBucket() (err error) {
	if bs.storageType == config.StorageTypeAliyun {
		err = bs.initAliyunBucket()
	} else if bs.storageType == config.StorageTypeTencent {
		err = bs.initTencentBucket()
	} else if bs.storageType == config.StorageTypeQiniu {
		err = bs.initQiniuBucket()
	} else if bs.storageType == config.StorageTypeUpyun {
		err = bs.initUpyunBucket()
	} else {
		bs.storageType = config.StorageTypeLocal
	}

	return err
}

func (bs *BucketStorage) UploadFile(location string, buff []byte) (string, error) {
	location = strings.TrimLeft(location, "/")
	if config.JsonData.PluginStorage.KeepLocal || bs.storageType == config.StorageTypeLocal {
		//log.Println("本地存储", location)
		//将文件写入本地
		basePath := config.ExecPath + "public/"
		//先判断文件夹是否存在，不存在就先创建
		_, err := os.Stat(basePath + location)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Dir(basePath+location), os.ModePerm)
			if err != nil {
				return "", err
			}
		}
		err = ioutil.WriteFile(basePath+location, buff, os.ModePerm)
		if err != nil {
			log.Println(err.Error())
			//无法创建
			return "", err
		}
	}
	if bs.storageType == config.StorageTypeAliyun {
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
	} else if bs.storageType == config.StorageTypeTencent {
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
	} else if bs.storageType == config.StorageTypeQiniu {
		//log.Println("使用七牛云上传")
		if bs.qiniuBucketClient == nil {
			err := bs.initQiniuBucket()
			if err != nil {
				return "", err
			}
		}
		putPolicy := storage.PutPolicy{
			Scope: fmt.Sprintf("%s:%s", config.JsonData.PluginStorage.QiniuBucket, location),
		}
		upToken := putPolicy.UploadToken(bs.qiniuBucketClient)

		cfg := storage.Config{}
		// 是否使用https域名
		//cfg.UseHTTPS = true
		// 上传是否使用CDN上传加速
		cfg.UseCdnDomains = false
		region, _ := storage.GetRegionByID(storage.RegionID(config.JsonData.PluginStorage.QiniuRegion))
		cfg.Zone = &region
		formUploader := storage.NewFormUploader(&cfg)
		ret := storage.PutRet{}
		putExtra := storage.PutExtra{}
		err := formUploader.Put(context.Background(), &ret, upToken, location, bytes.NewReader(buff), -1, &putExtra)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
	} else if bs.storageType == config.StorageTypeUpyun {
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
	u, _ := url.Parse(config.JsonData.PluginStorage.TencentBucketUrl)
	b := &cos.BaseURL{BucketURL: u}
	// Permanent key
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.JsonData.PluginStorage.TencentSecretId,
			SecretKey: config.JsonData.PluginStorage.TencentSecretKey,
		},
	})

	bs.tencentBucketClient = client

	return nil
}

func (bs *BucketStorage) initAliyunBucket() error {
	client, err := oss.New(config.JsonData.PluginStorage.AliyunEndpoint, config.JsonData.PluginStorage.AliyunAccessKeyId, config.JsonData.PluginStorage.AliyunAccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(config.JsonData.PluginStorage.AliyunBucketName)
	if err != nil {
		return err
	}

	bs.aliyunBucketClient = bucket

	return nil
}

func (bs *BucketStorage) initQiniuBucket() error {
	mac := qbox.NewMac(config.JsonData.PluginStorage.QiniuAccessKey, config.JsonData.PluginStorage.QiniuSecretKey)

	bs.qiniuBucketClient = mac

	return nil
}

func (bs *BucketStorage) initUpyunBucket() error {
	up := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   config.JsonData.PluginStorage.UpyunBucket,
		Operator: config.JsonData.PluginStorage.UpyunOperator,
		Password: config.JsonData.PluginStorage.UpyunPassword,
	})

	bs.upyunBucketClient = up

	return nil
}

func init() {
	var err error
	Storage, err = GetBucket()

	if err != nil {
		// 退回到local
		log.Println(err.Error())
		Storage.storageType = config.StorageTypeLocal
	}
}
