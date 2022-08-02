import React, { useEffect, useState } from 'react';
import ProForm, { ProFormRadio, ProFormText, ProFormTextArea } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Button, Card, Divider, message, Tabs } from 'antd';
import { pluginGetStorage, pluginSaveStorage } from '@/services';
import { useModel } from 'umi';

const PluginStorage: React.FC<any> = (props) => {
  const { initialState } = useModel('@@initialState');
  const [pushSetting, setPushSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetStorage();
    let setting = res.data || {};
    setPushSetting(setting);
    setFetched(true);
  };

  const onSubmit = async (values: any) => {
    const hide = message.loading('正在提交中', 0);
    pluginSaveStorage(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };
  return (
    <PageHeaderWrapper>
      <Card>
        <Alert message="资源存储方式的切换不会自动同步之前已经上传的资源，一般不建议在使用中切换存储方式。" />
        <div className="center-mid-card">
          {fetched && (
            <ProForm onFinish={onSubmit} initialValues={pushSetting}>
              <Divider>基本配置</Divider>
              <ProFormRadio.Group
                name="storage_type"
                label="存储方式"
                options={[
                  {
                    value: 'local',
                    label: '本地存储',
                  },
                  {
                    value: 'aliyun',
                    label: '阿里云存储',
                  },
                  {
                    value: 'tencent',
                    label: '腾讯云存储',
                  },
                  {
                    value: 'qiniu',
                    label: '七牛云存储',
                  },
                ]}
              />
              <ProFormText name="storage_url" label="资源地址" placeholder="" />
              <ProFormRadio.Group
                name="keep_local"
                label="本地存档"
                options={[
                  {
                    value: false,
                    label: '不保留',
                  },
                  {
                    value: true,
                    label: '保留',
                  },
                ]}
                extra="使用云存储的时候，可以选择保留本地存档"
              />
              <Divider>阿里云存储</Divider>
              <ProFormText
                name="aliyun_endpoint"
                label="阿里云节点"
                placeholder="例如：http://oss-cn-hangzhou.aliyuncs.com"
              />
              <ProFormText name="aliyun_access_key_id" label="阿里云AccessKeyId" placeholder="" />
              <ProFormText
                name="aliyun_access_key_secret"
                label="阿里云AccessKeySecret"
                placeholder=""
              />
              <ProFormText name="aliyun_bucket_name" label="阿里云存储桶名称" placeholder="" />
              <Divider>腾讯云存储</Divider>
              <ProFormText name="tencent_secret_id" label="腾讯云SecretId" placeholder="" />
              <ProFormText name="tencent_secret_key" label="腾讯云SecretKey" placeholder="" />
              <ProFormText
                name="tencent_bucket_url"
                label="腾讯云存储桶地址"
                placeholder="例如：https://aa-1257021234.cos.ap-guangzhou.myqcloud.com"
              />
              <Divider>七牛云存储</Divider>
              <ProFormText name="qiniu_access_key" label="七牛云AccessKey" placeholder="" />
              <ProFormText name="qiniu_secret_key" label="七牛云SecretKey" placeholder="" />
              <ProFormText
                name="qiniu_bucket"
                label="七牛云存储桶名称"
                placeholder="例如：anqicms"
              />
              <ProFormRadio.Group
                name="qiniu_region"
                label="七牛云存储区域"
                options={[
                  {
                    value: 'z0',
                    label: '华东',
                  },
                  {
                    value: 'z1',
                    label: '华北',
                  },
                  {
                    value: 'z2',
                    label: '华南',
                  },
                  {
                    value: 'na0',
                    label: '北美',
                  },
                  {
                    value: 'as0',
                    label: '东南亚',
                  },
                  {
                    value: 'cn-east-2',
                    label: '华东-浙江2',
                  },
                  {
                    value: 'fog-cn-east-1',
                    label: '雾存储华东区',
                  }
                ]}
              />
            </ProForm>
          )}
        </div>
      </Card>
    </PageHeaderWrapper>
  );
};

export default PluginStorage;
