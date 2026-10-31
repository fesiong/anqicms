import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  pluginGetStorage,
  pluginSaveStorage,
  pluginStorageUploadFile,
} from '@/services';
import {
  ProForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Divider, Upload, message } from 'antd';
import React, { useEffect, useState } from 'react';

const PluginStorage: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const [pushSetting, setPushSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [storageType, setStorageType] = useState<string>('local');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetStorage();
    let setting = res.data || {};
    setPushSetting(setting);
    setStorageType(setting.storage_type);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const changeStorageType = (e: any) => {
    setStorageType(e.target.value);
  };

  const handleUploadFile = (field: string, e: any) => {
    const formData = new FormData();
    formData.append('file', e.file);
    formData.append('name', field);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginStorageUploadFile(formData)
      .then((res) => {
        message.success(res.msg);
        pushSetting[field] = res.data;
        setPushSetting(Object.assign({}, pushSetting));
      })
      .finally(() => {
        hide();
      });
  };

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
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
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <Alert message={intl.formatMessage({ id: 'plugin.storage.tips' })} />
        <div className="center-mid-card">
          {fetched && (
            <ProForm onFinish={onSubmit} initialValues={pushSetting}>
              <Divider>
                <FormattedMessage id="plugin.storage.base" />
              </Divider>
              <ProFormRadio.Group
                name="storage_type"
                label={intl.formatMessage({ id: 'plugin.storage.type' })}
                fieldProps={{
                  onChange: changeStorageType,
                }}
                disabled={isVip === false}
                extra={
                  !isVip ? (
                    <div
                      className="link"
                      onClick={() => {
                        checkVip(() => {});
                      }}
                    >
                      <FormattedMessage id="plugin.storage.vip-tip" />
                    </div>
                  ) : null
                }
                options={[
                  {
                    value: 'local',
                    label: intl.formatMessage({
                      id: 'plugin.storage.type.local',
                    }),
                  },
                  {
                    value: 'aliyun',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.aliyun',
                    }),
                  },
                  {
                    value: 'tencent',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.tencent',
                    }),
                  },
                  {
                    value: 'qiniu',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.qiniu',
                    }),
                  },
                  {
                    value: 'upyun',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.upyun',
                    }),
                  },
                  {
                    value: 'google',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.google',
                    }),
                  },
                  {
                    value: 'awss3',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.awss3',
                    }),
                  },
                  {
                    value: 'r2',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.r2',
                    }),
                  },
                  {
                    value: 'ftp',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.ftp',
                    }),
                  },
                  {
                    value: 'ssh',
                    label: intl.formatMessage({
                      id: 'plugin.htmlcache.storage-type.ssh',
                    }),
                  },
                ]}
              />
              <ProFormText
                name="storage_url"
                label={intl.formatMessage({ id: 'plugin.storage.url' })}
                placeholder=""
              />
              <div className={storageType === 'local' ? 'hidden' : ''}>
                <ProFormRadio.Group
                  name="keep_local"
                  label={intl.formatMessage({
                    id: 'plugin.storage.keep-local',
                  })}
                  options={[
                    {
                      value: false,
                      label: intl.formatMessage({
                        id: 'plugin.storage.keep-local.no',
                      }),
                    },
                    {
                      value: true,
                      label: intl.formatMessage({
                        id: 'plugin.storage.keep-local.yes',
                      }),
                    },
                  ]}
                  extra={intl.formatMessage({
                    id: 'plugin.storage.keep-local.description',
                  })}
                />
              </div>
              <div className={storageType !== 'aliyun' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.aliyun" />
                </Divider>
                <ProFormText
                  name="aliyun_endpoint"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.aliyun.endpoint',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.aliyun.endpoint.placeholder',
                  })}
                />
                <ProFormText
                  name="aliyun_access_key_id"
                  label="AccessKeyId"
                  placeholder=""
                />
                <ProFormText
                  name="aliyun_access_key_secret"
                  label="AccessKeySecret"
                  placeholder=""
                />
                <ProFormText
                  name="aliyun_bucket_name"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.aliyun.bucket-name',
                  })}
                  placeholder=""
                />
              </div>
              <div className={storageType !== 'tencent' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.tencent" />
                </Divider>
                <ProFormText
                  name="tencent_secret_id"
                  label="SecretId"
                  placeholder=""
                />
                <ProFormText
                  name="tencent_secret_key"
                  label="SecretKey"
                  placeholder=""
                />
                <ProFormText
                  name="tencent_bucket_url"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.tencent.bucket-url',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.tencent.bucket-url.placeholder',
                  })}
                />
              </div>
              <div className={storageType !== 'qiniu' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.qiniu" />
                </Divider>
                <ProFormText
                  name="qiniu_access_key"
                  label="AccessKey"
                  placeholder=""
                />
                <ProFormText
                  name="qiniu_secret_key"
                  label="SecretKey"
                  placeholder=""
                />
                <ProFormText
                  name="qiniu_bucket"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.qiniu.bucket-name',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.qiniu.bucket-name.placeholder',
                  })}
                />
                <ProFormRadio.Group
                  name="qiniu_region"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.qiniu.region',
                  })}
                  options={[
                    {
                      value: 'z0',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.z0',
                      }),
                    },
                    {
                      value: 'z1',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.z1',
                      }),
                    },
                    {
                      value: 'z2',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.z2',
                      }),
                    },
                    {
                      value: 'na0',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.na0',
                      }),
                    },
                    {
                      value: 'as0',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.as0',
                      }),
                    },
                    {
                      value: 'cn-east-2',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.cn-east2',
                      }),
                    },
                    {
                      value: 'fog-cn-east-1',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.qiniu.region.fog-cn-east1',
                      }),
                    },
                  ]}
                />
              </div>
              <div className={storageType !== 'upyun' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.upyun" />
                </Divider>
                <ProFormText
                  name="upyun_operator"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.upyun.operator',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="upyun_password"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.upyun.password',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="upyun_bucket"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.upyun.bucket',
                  })}
                  placeholder=""
                />
              </div>
              <div className={storageType !== 'google' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.google" />
                </Divider>
                <ProFormText
                  name="google_project_id"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.google.project_id',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.google.project_id.placeholder',
                  })}
                />
                <ProFormText
                  name="google_bucket_name"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.google.bucket_name',
                  })}
                  placeholder=""
                />
                <ProFormTextArea
                  name="google_credentials_json"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.google.credentials_json',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.google.credentials_json.placeholder',
                  })}
                />
              </div>
              <div
                className={
                  storageType !== 'awss3' && storageType !== 'r2'
                    ? 'hidden'
                    : ''
                }
              >
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.awss3" />
                </Divider>
                {storageType === 'r2' && (
                  <ProFormText name="s3_endpoint" label="Endpoint" />
                )}
                {storageType === 'awss3' && (
                  <ProFormText
                    name="s3_region"
                    label={intl.formatMessage({
                      id: 'plugin.htmlcache.awss3.s3_region',
                    })}
                    placeholder={intl.formatMessage({
                      id: 'plugin.htmlcache.awss3.s3_region.placeholder',
                    })}
                  />
                )}
                <ProFormText
                  name="s3_region"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.awss3.s3_region',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.awss3.s3_region.placeholder',
                  })}
                />
                <ProFormText
                  name="s3_access_key"
                  label="AccessKey"
                  placeholder=""
                />
                <ProFormText
                  name="s3_secret_key"
                  label="SecretKey"
                  placeholder=""
                />
                <ProFormText
                  name="s3_bucket"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.awss3.s3_bucket',
                  })}
                  placeholder=""
                />
              </div>
              <div className={storageType !== 'ftp' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.ftp" />
                </Divider>
                <p>
                  <FormattedMessage id="plugin.htmlcache.ftp.tips" />
                </p>
                <ProFormText
                  name="ftp_host"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ftp.host',
                  })}
                  placeholder=""
                />
                <ProFormDigit
                  name="ftp_port"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ftp.port',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="ftp_username"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ftp.username',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="ftp_password"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ftp.password',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="ftp_webroot"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ftp.webroot',
                  })}
                  placeholder=""
                />
              </div>
              <div className={storageType !== 'ssh' ? 'hidden' : ''}>
                <Divider>
                  <FormattedMessage id="plugin.htmlcache.storage-type.ssh" />
                </Divider>
                <ProFormText
                  name="ssh_host"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.host',
                  })}
                  placeholder=""
                />
                <ProFormDigit
                  name="ssh_port"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.port',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="ssh_username"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.username',
                  })}
                  placeholder=""
                />
                <ProFormText
                  name="ssh_password"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.password',
                  })}
                  placeholder=""
                />
                <ProFormText
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.or-key',
                  })}
                  extra={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.or-key.description',
                  })}
                >
                  <Upload
                    name="file"
                    className="logo-uploader"
                    showUploadList={false}
                    accept=".crt,.pem,.key"
                    customRequest={async (e) =>
                      handleUploadFile('ssh_private_key', e)
                    }
                  >
                    <Button type="primary">
                      <FormattedMessage id="plugin.htmlcache.ssh.or-key.upload" />
                    </Button>
                  </Upload>
                  {pushSetting.ssh_private_key && (
                    <div className="upload-file">
                      {pushSetting.ssh_private_key}
                    </div>
                  )}
                </ProFormText>
                <ProFormText
                  name="ssh_webroot"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.ssh.webroot',
                  })}
                  placeholder=""
                />
              </div>
            </ProForm>
          )}
        </div>
      </Card>
      <VipModal />
    </NewContainer>
  );
};

export default PluginStorage;
