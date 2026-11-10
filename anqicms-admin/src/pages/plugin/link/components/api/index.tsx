import { useVipModal } from '@/components/vipModal';
import { pluginGetImportApiSetting, pluginUpdateApiToken } from '@/services';
import { ModalForm, ProCard, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
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

const PluginLinkApi: React.FC<any> = (props) => {
  const { checkVip, VipModal } = useVipModal();
  const [visible, setVisible] = useState<boolean>(false);
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
    if (values.link_token === '') {
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

  const handleCopied = () => {
    message.success(
      intl.formatMessage({ id: 'plugin.importapi.token.copy.success' }),
    );
  };

  const handleShowApi = () => {
    checkVip(() => setVisible(true));
  };

  return (
    <>
      <VipModal />
      <div
        onClick={() => {
          handleShowApi();
        }}
      >
        {props.children}
      </div>
      <Modal
        width={1000}
        zIndex={10}
        open={visible}
        onCancel={() => {
          setVisible(false);
        }}
        footer={null}
        title={intl.formatMessage({ id: 'plugin.link.api.title' })}
      >
        <Alert
          message={
            <div>
              <div>
                <Space>
                  <span>
                    <FormattedMessage id="plugin.importapi.token.name" />
                  </span>
                  <Tag>
                    <CopyToClipboard
                      text={setting.link_token}
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        <div className="token-message">
                          {setting.link_token}
                        </div>
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
              tab={intl.formatMessage({ id: 'plugin.link.api.list' })}
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
                        '/api/friendlink/list?token=' +
                        setting.link_token
                      }
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.base_url}/api/friendlink/list?token=
                        {setting.link_token}
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
                    <FormattedMessage id="plugin.importapi.category-api.fields.empty" />
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
                            {`{\n    "code": 200,\n    "msg": "",\n    "data": {\n      [\n        {\n          "id": 1,\n          "link": "https://www.anqicms.com/",\n          "title": "AnQiCMS",\n        },\n        {\n          "id": 2,\n          "link": "https://www.baidu.com/",\n          "title": "百度",\n        }\n      ]\n    }\n}`}
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
                            {`{\n    "code": -1,\n    "msg": "Token错误"\n}`}
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
              tab={intl.formatMessage({ id: 'plugin.link.api.verify' })}
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
                        '/api/friendlink/check?token=' +
                        setting.link_token
                      }
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.base_url}/api/friendlink/check?token=
                        {setting.link_token}
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
                    <FormattedMessage id="plugin.importapi.category-api.fields.empty" />
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
                          <code>{`{\n    "code": 200,\n    "msg": ""\n}`}</code>
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
              tab={intl.formatMessage({ id: 'plugin.link.api.add' })}
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
                        '/api/friendlink/create?token=' +
                        setting.link_token
                      }
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.base_url}/api/friendlink/create?token=
                        {setting.link_token}
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
                          title: 'title',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.other-title',
                          }),
                        },
                        {
                          title: 'link',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.other-link',
                          }),
                        },
                        {
                          title: 'nofollow',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.nofollow',
                          }),
                        },
                        {
                          title: 'back_link',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.back-link',
                          }),
                        },
                        {
                          title: 'my_title',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.self-title',
                          }),
                        },
                        {
                          title: 'my_link',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.self-link',
                          }),
                        },
                        {
                          title: 'contact',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.contact',
                          }),
                        },
                        {
                          title: 'remark',
                          required: false,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.remark',
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
                            {`{\n    "code": 200,\n    "msg": "链接已保存",\n}`}
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
              key="4"
              tab={intl.formatMessage({ id: 'plugin.link.api.delete' })}
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
                        '/api/friendlink/delete?token=' +
                        setting.link_token
                      }
                      onCopy={handleCopied}
                    >
                      <Tooltip
                        title={intl.formatMessage({
                          id: 'plugin.importapi.token.copy',
                        })}
                      >
                        {setting.base_url}/api/friendlink/delete?token=
                        {setting.link_token}
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
                          title: 'link',
                          required: true,
                          remark: intl.formatMessage({
                            id: 'plugin.link.field.other-link',
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
                            {`{\n    "code": 200,\n    "msg": "链接已删除"\n}`}
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
                            {`{\n    "code": -1, \n    "msg": "Token错误",\n}`}
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
          </ProCard>
        </div>
      </Modal>
      <ModalForm
        width={600}
        modalProps={{
          zIndex: 100,
        }}
        title={'plugin.importapi.token.reset'}
        open={tokenVidible}
        layout="horizontal"
        onOpenChange={(flag) => {
          setTokenVisible(flag);
        }}
        onFinish={handleUpdateToken}
      >
        <ProFormText
          name="link_token"
          label="plugin.importapi.token.new"
          placeholder={'plugin.importapi.token.new.placeholder'}
          extra="plugin.importapi.token.new.description"
        />
      </ModalForm>
    </>
  );
};
export default PluginLinkApi;
