import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  pluginBuildHtmlArchiveCache,
  pluginBuildHtmlCache,
  pluginBuildHtmlCategoryCache,
  pluginBuildHtmlIndexCache,
  pluginBuildHtmlTagCache,
  pluginCleanHtmlCache,
  pluginGetBuildHtmlCacheStatus,
  pluginGetHtmlCache,
  pluginGetHtmlCachePushStatus,
  pluginHtmlCachePush,
  pluginHtmlCacheUploadFile,
  pluginSaveHtmlCache,
} from '@/services';
import {
  ProForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Modal,
  Row,
  Space,
  Upload,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import HtmlPushLog from './components/pushLog';

let xhr: any = null;
let pushXhr: any = null;

const PluginHtmlCache: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const [status, setStatus] = useState<any>(null);
  const [pushStatus, setPushStatus] = useState<any>(null);
  const [setting, setSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [storageType, setStorageType] = useState<string>('local');
  const [logVisible, setLogVisible] = useState<boolean>(false);
  const [logStatus, setLogStatus] = useState<string>('');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetHtmlCache();
    let setting = res.data || {};
    setSetting(setting);
    setStorageType(setting.storage_type);
    setFetched(true);
  };

  const changeStorageType = (e: any) => {
    setStorageType(e.target.value);
  };

  const getPushStatus = () => {
    pluginGetHtmlCachePushStatus().then((res) => {
      setPushStatus(res.data || null);
      if (!res.data || res.data.finished_time > 0) {
        clearInterval(pushXhr);
        return;
      }
    });
  };

  const startCheckPushStatus = () => {
    clearInterval(pushXhr);
    pushXhr = setInterval(() => {
      getPushStatus();
    }, 1500);
  };

  const getStatus = () => {
    pluginGetBuildHtmlCacheStatus().then((res) => {
      setStatus(res.data || null);
      if (!res.data || res.data.finished_time > 0) {
        clearInterval(xhr);
        startCheckPushStatus();
        return;
      }
    });
  };

  const onTabChange = (key: string) => {
    clearInterval(xhr);
    clearInterval(pushXhr);
    getSetting().then(() => {
      getStatus();
      getPushStatus();
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
    getStatus();
    getPushStatus();
    return () => {
      clearInterval(xhr);
      clearInterval(pushXhr);
    };
  }, []);

  const startBuild = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.htmlcache.generate.all.confirm',
      }),
      onOk: () => {
        pluginBuildHtmlCache().then((res) => {
          message.info(res.msg);
          getStatus();
          xhr = setInterval(() => {
            getStatus();
          }, 1500);
        });
      },
    });
  };

  const startBuildIndex = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.htmlcache.generate.home.confirm',
      }),
      onOk: () => {
        pluginBuildHtmlIndexCache().then((res) => {
          message.info(res.msg);
          getStatus();
          xhr = setInterval(() => {
            getStatus();
          }, 1500);
        });
      },
    });
  };

  const startBuildCategory = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.htmlcache.generate.category.confirm',
      }),
      onOk: () => {
        pluginBuildHtmlCategoryCache().then((res) => {
          message.info(res.msg);
          getStatus();
          xhr = setInterval(() => {
            getStatus();
          }, 1500);
        });
      },
    });
  };

  const startBuildArchive = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.htmlcache.generate.archive.confirm',
      }),
      onOk: () => {
        pluginBuildHtmlArchiveCache().then((res) => {
          message.info(res.msg);
          getStatus();
          xhr = setInterval(() => {
            getStatus();
          }, 1500);
        });
      },
    });
  };

  const startBuildTag = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.htmlcache.generate.tag.confirm',
      }),
      onOk: () => {
        pluginBuildHtmlTagCache().then((res) => {
          message.info(res.msg);
          getStatus();
          xhr = setInterval(() => {
            getStatus();
          }, 1500);
        });
      },
    });
  };

  const cleanHtmlCache = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.htmlcache.clean.confirm' }),
      content: intl.formatMessage({
        id: 'plugin.htmlcache.clean.confirm.content',
      }),
      onOk: () => {
        pluginCleanHtmlCache().then(() => {
          message.success(
            intl.formatMessage({ id: 'plugin.htmlcache.clean.success' }),
          );
        });
      },
    });
  };

  const handleUploadFile = (field: string, e: any) => {
    const formData = new FormData();
    formData.append('file', e.file);
    formData.append('name', field);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginHtmlCacheUploadFile(formData)
      .then((res) => {
        message.success(res.msg);
        setting[field] = res.data;
        setSetting(Object.assign({}, setting));
      })
      .finally(() => {
        hide();
      });
  };

  const startPushAll = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.htmlcache.push.all.confirm' }),
      content: intl.formatMessage({
        id: 'plugin.htmlcache.push.all.confirm.content',
      }),
      onOk: () => {
        pluginHtmlCachePush({
          all: true,
        }).then((res) => {
          message.info(res.msg);
          startCheckPushStatus();
        });
      },
    });
  };

  const startPushUpdate = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.htmlcache.push.addon.confirm' }),
      content: intl.formatMessage({
        id: 'plugin.htmlcache.push.addon.confirm.content',
      }),
      onOk: () => {
        pluginHtmlCachePush({
          all: false,
        }).then((res) => {
          message.info(res.msg);
          startCheckPushStatus();
        });
      },
    });
  };

  const getPushLog = () => {
    setLogStatus('');
    setLogVisible(true);
  };

  const getPushErrorLog = () => {
    setLogStatus('error');
    setLogVisible(true);
  };

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveHtmlCache(values)
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
        <Alert
          style={{ marginBottom: 20 }}
          description={
            <div>
              <p>
                <FormattedMessage id="plugin.htmlcache.description.1" />
              </p>
              <p>
                <FormattedMessage id="plugin.htmlcache.description.2" />
              </p>
              <p>
                <FormattedMessage id="plugin.htmlcache.description.3" />
              </p>
              <div>
                <FormattedMessage id="plugin.htmlcache.description.4" />
              </div>
            </div>
          }
        />
        {fetched && (
          <ProForm
            initialValues={setting}
            onFinish={onSubmit}
            title={intl.formatMessage({ id: 'menu.plugin.system.htmlcache' })}
          >
            <Row gutter={16}>
              <Col sm={10} xs={24}>
                <ProFormRadio.Group
                  name={'open'}
                  label={intl.formatMessage({ id: 'plugin.htmlcache.isopen' })}
                  options={[
                    {
                      label: intl.formatMessage({
                        id: 'plugin.fulltext.open.false',
                      }),
                      value: false,
                    },
                    {
                      label: intl.formatMessage({
                        id: 'plugin.fulltext.open.true',
                      }),
                      value: true,
                    },
                  ]}
                />

                <ProFormDigit
                  name="index_cache"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.index-time',
                  })}
                  wrapperCol={{ span: 12 }}
                  fieldProps={{
                    addonAfter: intl.formatMessage({
                      id: 'plugin.htmlcache.index-time.suffix',
                    }),
                  }}
                  extra={intl.formatMessage({
                    id: 'plugin.htmlcache.index-time.description',
                  })}
                />
                <ProFormDigit
                  name="category_cache"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.category-time',
                  })}
                  wrapperCol={{ span: 12 }}
                  fieldProps={{
                    addonAfter: intl.formatMessage({
                      id: 'plugin.htmlcache.index-time.suffix',
                    }),
                  }}
                  extra={intl.formatMessage({
                    id: 'plugin.htmlcache.index-time.description',
                  })}
                />
                <ProFormDigit
                  name="detail_cache"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.archive-time',
                  })}
                  wrapperCol={{ span: 12 }}
                  fieldProps={{
                    addonAfter: intl.formatMessage({
                      id: 'plugin.htmlcache.index-time.suffix',
                    }),
                  }}
                  extra={intl.formatMessage({
                    id: 'plugin.htmlcache.index-time.description',
                  })}
                />
              </Col>
              <Col sm={10} xs={24}>
                <ProFormRadio.Group
                  name="storage_type"
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.storage-type',
                  })}
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
                        <FormattedMessage id="plugin.htmlcache.vip-tip" />
                      </div>
                    ) : null
                  }
                  options={[
                    {
                      value: '',
                      label: intl.formatMessage({
                        id: 'plugin.htmlcache.storage-type.close',
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
                  label={intl.formatMessage({
                    id: 'plugin.htmlcache.storage-url',
                  })}
                  placeholder={intl.formatMessage({
                    id: 'plugin.htmlcache.storage-url.placeholder',
                  })}
                />
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
                <div className={storageType !== 'awss3' ? 'hidden' : ''}>
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
                    {setting.ssh_private_key && (
                      <div className="upload-file">
                        {setting.ssh_private_key}
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
              </Col>
            </Row>
          </ProForm>
        )}
        <Divider>
          <FormattedMessage id="plugin.htmlcache.generate.name" />
        </Divider>
        <div>
          <p>
            <FormattedMessage id="plugin.htmlcache.generate.last-time" />
            {setting.last_build_time > 0
              ? dayjs(setting.last_build_time * 1000).format('YYYY-MM-DD')
              : intl.formatMessage({
                  id: 'plugin.htmlcache.generate.last-time.empty',
                })}
          </p>
          <Space className="space-wrap" size={20}>
            <Button onClick={() => cleanHtmlCache()}>
              <FormattedMessage id="plugin.htmlcache.clean.all" />
            </Button>
            <Button onClick={() => startBuild()}>
              <FormattedMessage id="plugin.htmlcache.build.all" />
            </Button>
            <Button onClick={() => startBuildIndex()}>
              <FormattedMessage id="plugin.htmlcache.build.home" />
            </Button>
            <Button onClick={() => startBuildCategory()}>
              <FormattedMessage id="plugin.htmlcache.build.category" />
            </Button>
            <Button onClick={() => startBuildArchive()}>
              <FormattedMessage id="plugin.htmlcache.build.archive" />
            </Button>
            <Button onClick={() => startBuildTag()}>
              <FormattedMessage id="plugin.htmlcache.build.tag" />
            </Button>
          </Space>
        </div>
        <Divider>
          <FormattedMessage id="plugin.htmlcache.push.name" />
        </Divider>
        <p>
          <FormattedMessage id="plugin.htmlcache.push.last-time" />
          {setting.last_push_time > 0
            ? dayjs(setting.last_push_time * 1000).format('YYYY-MM-DD')
            : intl.formatMessage({
                id: 'plugin.htmlcache.push.last-time.empty',
              })}
        </p>
        <div>
          <Space className="space-wrap" size={20}>
            <Button onClick={() => startPushAll()}>
              <FormattedMessage id="plugin.htmlcache.push.all" />
            </Button>
            <Button onClick={() => startPushUpdate()}>
              <FormattedMessage id="plugin.htmlcache.push.addon" />
            </Button>
            <Button onClick={() => getPushLog()}>
              <FormattedMessage id="plugin.htmlcache.push.log.all" />
            </Button>
            <Button onClick={() => getPushErrorLog()}>
              <FormattedMessage id="plugin.htmlcache.push.log.error" />
            </Button>
          </Space>
        </div>
        {(status || pushStatus) && (
          <Row gutter={16}>
            {status && (
              <Col sm={12} xs={24}>
                <div>
                  <Divider>
                    <FormattedMessage id="plugin.htmlcache.build.process" />
                  </Divider>
                  <Space direction="vertical" size={10}>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.start-time" />
                      </div>
                      <div className="field-value">
                        {dayjs(status.start_time * 1000).format(
                          'YYYY-MM-DD HH:mm:ss',
                        )}
                      </div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.end-time" />
                      </div>
                      <div className="field-value">
                        {status.finished_time > 0
                          ? dayjs(status.finished_time * 1000).format(
                              'YYYY-MM-DD HH:mm:ss',
                            )
                          : intl.formatMessage({
                              id: 'plugin.htmlcache.build.unfinished',
                            })}
                      </div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.total" />
                      </div>
                      <div className="field-value">{status.total}</div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.finished-count" />
                      </div>
                      <div className="field-value">{status.finished_count}</div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.current" />
                      </div>
                      <div className="field-value">{status.current}</div>
                    </div>
                    {status.error_msg && (
                      <div className="field-item">
                        <div className="field-label">
                          <FormattedMessage id="plugin.htmlcache.build.error-msg" />
                        </div>
                        <div className="field-value">{status.error_msg}</div>
                      </div>
                    )}
                  </Space>
                </div>
              </Col>
            )}
            {pushStatus && (
              <Col sm={12} xs={24}>
                <div>
                  <Divider>
                    <FormattedMessage id="plugin.htmlcache.push.process" />
                  </Divider>
                  <Space direction="vertical" size={10}>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.start-time" />
                      </div>
                      <div className="field-value">
                        {dayjs(pushStatus.start_time * 1000).format(
                          'YYYY-MM-DD HH:mm:ss',
                        )}
                      </div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.end-time" />
                      </div>
                      <div className="field-value">
                        {pushStatus.finished_time > 0
                          ? dayjs(pushStatus.finished_time * 1000).format(
                              'YYYY-MM-DD HH:mm:ss',
                            )
                          : intl.formatMessage({
                              id: 'plugin.htmlcache.build.unfinished',
                            })}
                      </div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.total" />
                      </div>
                      <div className="field-value">{pushStatus.total}</div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.finished-count" />
                      </div>
                      <div className="field-value">
                        {pushStatus.finished_count}
                      </div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.error-count" />
                      </div>
                      <div className="field-value">
                        {pushStatus.error_count}
                      </div>
                    </div>
                    <div className="field-item">
                      <div className="field-label">
                        <FormattedMessage id="plugin.htmlcache.build.current" />
                      </div>
                      <div className="field-value">{pushStatus.current}</div>
                    </div>
                    {pushStatus.error_msg && (
                      <div className="field-item">
                        <div className="field-label">
                          <FormattedMessage id="plugin.htmlcache.build.error-msg" />
                        </div>
                        <div className="field-value">
                          {pushStatus.error_msg}
                        </div>
                      </div>
                    )}
                  </Space>
                </div>
              </Col>
            )}
          </Row>
        )}
      </Card>
      {logVisible && (
        <HtmlPushLog
          open={logVisible}
          status={logStatus}
          storageUrl={setting.storage_url}
          onCancel={() => {
            setLogVisible(false);
          }}
        />
      )}
      <VipModal />
    </NewContainer>
  );
};

export default PluginHtmlCache;
