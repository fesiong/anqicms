import trainImg from '@/images/train.png';
import { pluginGetImportApiSetting, pluginUpdateApiToken } from '@/services';
import { downloadFile } from '@/utils';
import {
  ModalForm,
  PageContainer,
  ProCard,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Modal,
  Space,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import CopyToClipboard from 'react-copy-to-clipboard';
import './index.less';

const PluginImportApi: React.FC<any> = () => {
  const [tokenVidible, setTokenVisible] = useState<boolean>(false);
  const [tab, setTab] = useState('1');
  const [setting, setSetting] = useState<any>({});
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetImportApiSetting();
    setSetting(res.data || {});
  };

  useEffect(() => {
    getSetting();
    let hash = history.location.hash || '';
    if (hash) {
      hash = hash.replaceAll('#', '');
      setTab(hash);
    }
  }, []);

  const handleUpdateToken = async (values: any) => {
    if (values.token === '') {
      message.error(
        intl.formatMessage({ id: 'plugin.importapi.token.required' }),
      );
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.importapi.token.confirm' }),
      content: intl.formatMessage({
        id: 'plugin.importapi.token.confirm.content',
      }),
      onOk: async () => {
        const res = await pluginUpdateApiToken(values);
        message.info(res.msg);
        getSetting();
        setTokenVisible(false);
      },
    });
  };

  const handleDownloadTrainModule = () => {
    downloadFile(
      '/plugin/transfer/download',
      {
        provider: 'train',
      },
      'train2anqicms.wpm',
    );
  };

  const handleCopied = () => {
    message.success(
      intl.formatMessage({ id: 'plugin.importapi.token.copy.success' }),
    );
  };

  return (
    <PageContainer>
      <Card>
        <Alert
          message={
            <div>
              <p>
                <FormattedMessage id="plugin.importapi.tips" />
              </p>
              <div>
                <Space>
                  <span>
                    <FormattedMessage id="plugin.importapi.token.name" />
                  </span>
                  <Tag>
                    <CopyToClipboard text={setting.token} onCopy={handleCopied}>
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </Tag>
                  <Button
                    size="small"
                    onClick={() => {
                      setTokenVisible(true);
                    }}
                  >
                    <FormattedMessage id="plugin.importapi.token.update" />
                  </Button>
                </Space>
              </div>
            </div>
          }
        />
        <div className="mt-normal">
          <ProCard
            tabs={{
              tabPosition: 'left',
              activeKey: tab,
              onChange: (key: string) => {
                setTab(key);
              },
            }}
          >
            <ProCard.TabPane
              key="1"
              tab={intl.formatMessage({ id: 'plugin.importapi.archive-api' })}
            >
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.api-url" />
                  </div>
                  <div className="value">
                    <CopyToClipboard
                      text={
                        setting.base_url +
                        '/api/import/archive?token=' +
                        setting.token
                      }
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.base_url}/api/import/archive?token=
                        {setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.method" />
                  </div>
                  <div className="value">POST</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.request-type" />
                  </div>
                  <div className="value">form-data</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.post-fields" />
                  </div>
                  <div className="value">
                    <Table
                      size="small"
                      pagination={false}
                      columns={[
                        {
                          title: intl.formatMessage({
                            id: 'content.module.field.name',
                          }),
                          dataIndex: 'title',
                          width: 150,
                        },
                        {
                          title: intl.formatMessage({
                            id: 'content.module.field.isrequired',
                          }),
                          dataIndex: 'required',
                          width: 100,
                          render: (text: number) => (
                            <span>
                              {text
                                ? intl.formatMessage({
                                    id: 'content.module.field.isrequired.yes',
                                  })
                                : intl.formatMessage({
                                    id: 'content.module.field.isrequired.no',
                                  })}
                            </span>
                          ),
                        },
                        {
                          title: intl.formatMessage({
                            id: 'plugin.importapi.field.remark',
                          }),
                          dataIndex: 'remark',
                        },
                      ]}
                      dataSource={[
                        {
                          title: 'id',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.archive-id',
                          }),
                        },
                        {
                          title: 'title',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.title',
                          }),
                        },
                        {
                          title: 'content',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.content',
                          }),
                        },
                        {
                          title: 'category_id',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.category-id',
                          }),
                        },
                        {
                          title: 'keywords',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.keywords',
                          }),
                        },
                        {
                          title: 'description',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.description',
                          }),
                        },
                        {
                          title: 'url_token',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.url-token',
                          }),
                        },
                        {
                          title: 'images[]',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.images',
                          }),
                        },
                        {
                          title: 'logo',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.logo',
                          }),
                        },
                        {
                          title: 'publish_time',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.publish-time',
                          }),
                        },
                        {
                          title: 'tag',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.tag',
                          }),
                        },
                        {
                          title: intl.formatMessage({
                            id: 'plugin.importapi.field.diy',
                          }),
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.diy.remark',
                          }),
                        },
                        {
                          title: 'draft',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.draft',
                          }),
                        },
                        {
                          title: 'cover',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.field.cover',
                          }),
                        },
                      ]}
                      key="title"
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.return-type" />
                  </div>
                  <div className="value">JSON</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.return-example.success" />
                  </div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {`{\n    "code": 200,\n    "msg": "发布成功",\n    "data": {\n        "url":"https://www.anqicms.com/..."\n    }\n}`}
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.return-example.failure" />
                  </div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {`{\n    "code": -1,\n    "msg": "Token错误",\n}`}
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
            <ProCard.TabPane
              key="3"
              tab={intl.formatMessage({ id: 'plugin.importapi.category-api' })}
            >
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.api-url" />
                  </div>
                  <div className="value">
                    <CopyToClipboard
                      text={
                        setting.base_url +
                        '/api/import/categories?token=' +
                        setting.token
                      }
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.base_url}/api/import/categories?token=
                        {setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.method" />
                  </div>
                  <div className="value">POST / GET</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.request-type" />
                  </div>
                  <div className="value">form-data / query params</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.category-api.fields" />
                  </div>
                  <div className="value">
                    <Table
                      size="small"
                      pagination={false}
                      columns={[
                        {
                          title: intl.formatMessage({
                            id: 'content.module.field.name',
                          }),
                          dataIndex: 'title',
                          width: 150,
                        },
                        {
                          title: intl.formatMessage({
                            id: 'content.module.field.isrequired',
                          }),
                          dataIndex: 'required',
                          width: 100,
                          render: (text: number) => (
                            <span>
                              {text
                                ? intl.formatMessage({
                                    id: 'content.module.field.isrequired.yes',
                                  })
                                : intl.formatMessage({
                                    id: 'content.module.field.isrequired.no',
                                  })}
                            </span>
                          ),
                        },
                        {
                          title: intl.formatMessage({
                            id: 'plugin.importapi.field.remark',
                          }),
                          dataIndex: 'remark',
                        },
                      ]}
                      dataSource={[
                        {
                          title: 'module_id',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.importapi.category-api.module-id',
                          }),
                        },
                      ]}
                      key="title"
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.return-type" />
                  </div>
                  <div className="value">JSON</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.return-example.success" />
                  </div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {`{\n    "code": 0,\n    "msg": "",\n    "data": {\n      [\n        {\n          "id": 1,\n          "parent_id": 0,\n          "title": "新闻大事",\n        },\n        {\n          "id": 2,\n          "parent_id": 1,\n          "title": "国际新闻",\n        },\n        {\n          "id": 3,\n          "parent_id": 1,\n          "title": "国内新闻",\n        },\n        {\n          "id": 4,\n          "parent_id": 0,\n          "title": "案例展示",\n        },\n      ]\n    }\n}`}
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.return-example.failure" />
                  </div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {`{\n    "code": -1,\n    "msg": "Token错误",\n}`}
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
            <ProCard.TabPane
              key="2"
              tab={intl.formatMessage({ id: 'plugin.importapi.train-mopdule' })}
            >
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.train-mopdule.url" />
                  </div>
                  <div className="value">{setting.base_url}</div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.train-mopdule.token" />
                  </div>
                  <div className="value">
                    <CopyToClipboard text={setting.token} onCopy={handleCopied}>
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.train-mopdule.download" />
                  </div>
                  <div className="value">
                    <Button onClick={handleDownloadTrainModule}>
                      <FormattedMessage id="plugin.importapi.train-mopdule.download.text" />{' '}
                      train2anqicms.wpm
                    </Button>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.train-mopdule.support-version" />
                  </div>
                  <div className="value">
                    <FormattedMessage id="plugin.importapi.train-mopdule.support-version.text" />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">
                    <FormattedMessage id="plugin.importapi.train-mopdule.example" />
                  </div>
                  <div className="value">
                    <img src={trainImg} />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
          </ProCard>
        </div>
      </Card>
      <ModalForm
        width={600}
        modalProps={{
          zIndex: 100,
        }}
        title={intl.formatMessage({ id: 'plugin.importapi.token.reset' })}
        open={tokenVidible}
        layout="horizontal"
        onOpenChange={(flag) => {
          setTokenVisible(flag);
        }}
        onFinish={handleUpdateToken}
      >
        <ProFormText
          name="token"
          label={intl.formatMessage({ id: 'plugin.importapi.token.new' })}
          placeholder={intl.formatMessage({
            id: 'plugin.importapi.token.new.placeholder',
          })}
          extra={intl.formatMessage({
            id: 'plugin.importapi.token.new.description',
          })}
        />
      </ModalForm>
    </PageContainer>
  );
};
export default PluginImportApi;
