import NewContainer from '@/components/NewContainer';
import { pluginReplaceValues, pluginBackupData, pluginGetBackupStatus } from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormCheckbox,
  ProFormInstance,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Divider,
  Input,
  Modal,
  Space,
  Tag,
  message,
} from 'antd';
import React, { useState } from 'react';
import './index.less';

const PluginReplace: React.FC<any> = () => {
  const formRef = React.createRef<ProFormInstance>();
  const [keywords, setKeywords] = useState<any[]>([]);
  const [inputVisible, setInputVisible] = useState<boolean>(false);
  const [fromValue, setFromValue] = useState<string>('');
  const [toValue, setToValue] = useState<string>('');
  const [newKey, setNewKey] = useState<string>('');
  const [backupBefore, setBackupBefore] = useState<boolean>(false);
  const [submitting, setSubmitting] = useState<boolean>(false);
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const handleRemove = (index: number) => {
    keywords.splice(index, 1);
    setKeywords([].concat(...keywords));
  };

  const handleEditInputChange = (field: string, e: any) => {
    if (field === 'from') {
      setFromValue(e.target.value);
    } else if (field === 'to') {
      setToValue(e.target.value);
    }
  };

  const handleEditInputConfirm = () => {
    if (!fromValue) {
      message.error(intl.formatMessage({ id: 'plugin.replace.add.required' }));
      return;
    }
    let tag: any = {
      from: fromValue,
      to: toValue,
    };
    keywords.push(tag);

    setKeywords([].concat(...keywords));
    setInputVisible(false);
  };

  const showInput = () => {
    setInputVisible(true);
    setFromValue('');
    setToValue('');
  };

  const waitBackupFinished = (onDone: () => void) => {
    const timer = setInterval(() => {
      pluginGetBackupStatus().then((res) => {
        if (!res.data || res.data.finished) {
          clearInterval(timer);
          onDone();
        }
      }).catch(() => {
        clearInterval(timer);
        onDone();
      });
    }, 1000);
  };

  const doReplace = (values: any) => {
    const postData = Object.assign({}, values);
    postData.keywords = keywords;
    setSubmitting(true);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginReplaceValues(postData)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
        setSubmitting(false);
      });
  };

  const onSubmit = async (values: any) => {
    if (!values.places || values.places.length === 0) {
      message.error(
        intl.formatMessage({ id: 'plugin.replace.place.required' }),
      );
      return;
    }
    if (keywords.length === 0) {
      message.error(
        intl.formatMessage({ id: 'plugin.replace.keyword.required' }),
      );
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.replace.confirm' }),
      onOk: () => {
        if (!backupBefore) {
          doReplace(values);
          return;
        }
        // 先执行备份
        setSubmitting(true);
        const hide = message.loading(
          intl.formatMessage({ id: 'plugin.replace.backuping' }),
          0,
        );
        pluginBackupData({})
          .then(() => {
            // 轮询备份状态，完成后执行替换
            waitBackupFinished(() => {
              hide();
              setSubmitting(false);
              doReplace(values);
            });
          })
          .catch((err) => {
            hide();
            setSubmitting(false);
            console.log(err);
            message.error(intl.formatMessage({ id: 'plugin.replace.backup.failed' }));
          });
      },
    });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <Alert
          message={
            <div>
              <p>
                <FormattedMessage id="plugin.aigenerate.replace.tips2" />
              </p>
              <p>
                <FormattedMessage id="plugin.aigenerate.replace.tips3" />
              </p>
              <p>
                <FormattedMessage id="plugin.aigenerate.replace.rules" />
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.email" />
                </Tag>
                、
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.date" />
                </Tag>
                、
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.time" />
                </Tag>
                、
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.cellphone" />
                </Tag>
                、
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.qq" />
                </Tag>
                、
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.wechat" />
                </Tag>
                、
                <Tag>
                  <FormattedMessage id="plugin.aigenerate.replace.rule.website" />
                </Tag>
              </p>
              <p className="text-red">
                <span className="text-red">*</span>{' '}
                <FormattedMessage id="plugin.aigenerate.replace.notice" />
                <br />
                <FormattedMessage id="plugin.replace.tips" />
              </p>
            </div>
          }
          type="info"
        />
        <div className="mt-normal">
          <ProForm onFinish={onSubmit} layout="horizontal" formRef={formRef}>
            <Card size="small" bordered={false}>
              <Divider orientation={'left'}>
                <FormattedMessage id="plugin.replace.place" />
              </Divider>
              <ProFormCheckbox.Group
                name="places"
                options={[
                  {
                    value: 'setting',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.setting',
                    }),
                  },
                  {
                    value: 'archive',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.archive',
                    }),
                  },
                  {
                    value: 'category',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.category',
                    }),
                  },
                  {
                    value: 'tag',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.tag',
                    }),
                  },
                  {
                    value: 'anchor',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.anchor',
                    }),
                  },
                  {
                    value: 'keyword',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.keyword',
                    }),
                  },
                  {
                    value: 'comment',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.comment',
                    }),
                  },
                  {
                    value: 'attachment',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.attachment',
                    }),
                  },
                  {
                    value: 'nav',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.nav',
                    }),
                  },
                  {
                    value: 'link',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.link',
                    }),
                  },
                  {
                    value: 'redirect',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.redirect',
                    }),
                  },
                  {
                    value: 'place',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.place',
                    }),
                  },
                  {
                    value: 'guestbook',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.guestbook',
                    }),
                  },
                  {
                    value: 'template',
                    label: intl.formatMessage({
                      id: 'plugin.replace.place.template',
                    }),
                  },
                ]}
              />
              <ProFormCheckbox
                label={intl.formatMessage({ id: 'plugin.replace.replace-tag' })}
                name="replace_tag"
              />
              <ProFormCheckbox
                label={intl.formatMessage({ id: 'plugin.replace.backup-before' })}
                name="backup_before"
                fieldProps={{
                  onChange: (e: any) => setBackupBefore(e.target.checked),
                }}
              />
              {backupBefore && (
                <div className="text-red" style={{ marginTop: -8, marginBottom: 12 }}>
                  <FormattedMessage id="plugin.replace.backup-before.tips" />
                </div>
              )}
              <Divider orientation={'left'}>
                <FormattedMessage id="plugin.replace.keyword" />
              </Divider>
              <div className="tag-lists">
                <Space size={[12, 12]} wrap>
                  {keywords.map((tag: any, index: number) => (
                    <div className="edit-tag" key={index}>
                      <span className="key">{tag.from}</span>
                      <span className="divide">
                        <FormattedMessage id="plugin.aigenerate.replace.to" />
                      </span>
                      <span className="value">
                        {tag.to ||
                          intl.formatMessage({ id: 'plugin.aigenerate.empty' })}
                      </span>
                      <span
                        className="close"
                        onClick={() => {
                          handleRemove(index);
                        }}
                      >
                        ×
                      </span>
                    </div>
                  ))}
                  {!inputVisible && (
                    <Button className="site-tag-plus" onClick={showInput}>
                      <PlusOutlined />{' '}
                      <FormattedMessage id="plugin.replace.add" />
                    </Button>
                  )}
                </Space>
              </div>
              {inputVisible && (
                <Input.Group compact>
                  <Input
                    style={{ width: '35%' }}
                    value={fromValue}
                    onChange={(e) => {
                      handleEditInputChange('from', e);
                    }}
                    onPressEnter={() => {
                      handleEditInputConfirm();
                    }}
                  />
                  <span className="input-divide">
                    <FormattedMessage id="plugin.aigenerate.replace.to" />
                  </span>
                  <Input
                    style={{ width: '35%' }}
                    value={toValue}
                    onChange={(e) => {
                      handleEditInputChange('to', e);
                    }}
                    onPressEnter={() => {
                      handleEditInputConfirm();
                    }}
                  />
                  <Button
                    onClick={() => {
                      handleEditInputConfirm();
                    }}
                    style={{ width: '15%', minWidth: '90px' }}
                  >
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </Button>
                </Input.Group>
              )}
              <p>
                <br />
              </p>
            </Card>
          </ProForm>
        </div>
      </Card>
    </NewContainer>
  );
};

export default PluginReplace;
