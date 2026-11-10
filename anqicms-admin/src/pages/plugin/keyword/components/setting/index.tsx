import { useVipModal } from '@/components/vipModal';
import { getKeywordSetting, saveKeywordSetting } from '@/services';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage } from '@umijs/max';
import { Input, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import { useIntl } from 'react-intl';
import './index.less';

export type KeywordSettingProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const KeywordSetting: React.FC<KeywordSettingProps> = (props) => {
  const { checkVip, VipModal } = useVipModal();
  const [visible, setVisible] = useState<boolean>(false);
  const [fetched, setFetched] = useState<boolean>(false);
  const [setting, setSetting] = useState<any>({});
  const [tmpInput, setTmpInput] = useState<any>({});
  const intl = useIntl();

  useEffect(() => {
    getKeywordSetting().then((res) => {
      let setting = res.data;
      if (!setting.title_exclude) {
        setting.title_exclude = [];
      }
      if (!setting.title_replace) {
        setting.title_replace = [];
      }
      setSetting(setting);
      setFetched(true);
    });
  }, []);

  const handleSetVisible = (visible: boolean) => {
    setVisible(visible);
  };

  const handleShowVisible = () => {
    checkVip(() => setVisible(true));
  };

  const handleSubmit = async (data: any) => {
    let values = Object.assign(setting, data);
    values.max_count = Number(values.max_count);

    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveKeywordSetting(values)
      .then((res) => {
        message.info(res.msg);
        handleSetVisible(false);
        props.onCancel();
      })
      .finally(() => {
        hide();
      });
  };

  const handleRemove = (field: string, index: number) => {
    setting[field].splice(index, 1);
    setSetting({ ...setting });
  };

  const handleChangeTmpInput = (field: string, e: any) => {
    tmpInput[field] = e.target.value;
    setTmpInput({ ...tmpInput });
  };

  const handleAddField = (field: string) => {
    if (field === 'title_replace') {
      if (!tmpInput['from'] || tmpInput['from'] === tmpInput['to']) {
        return;
      }
      let exists = false;
      for (let item of setting[field]) {
        if (item.from === tmpInput['from']) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        setting[field].push({
          from: tmpInput['from'],
          to: tmpInput['to'],
        });

        tmpInput['from'] = '';
        tmpInput['to'] = '';
      }
    } else {
      setting[field].push(tmpInput[field]);
      tmpInput[field] = '';
    }
    setTmpInput({ ...tmpInput });
    setSetting({ ...setting });
  };

  return (
    <>
      <VipModal />
      <div
        onClick={() => {
          handleShowVisible();
        }}
      >
        {props.children}
      </div>
      {fetched && (
        <ModalForm
          width={800}
          title={intl.formatMessage({
            id: 'plugin.keyword.dig-setting',
          })}
          initialValues={setting}
          open={visible}
          //layout="horizontal"
          onOpenChange={(flag) => {
            handleSetVisible(flag);
            if (!flag) {
              props.onCancel(flag);
            }
          }}
          onFinish={async (values) => {
            handleSubmit(values);
          }}
        >
          <ProFormRadio.Group
            name="auto_dig"
            label={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.auto-dig',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.keyword.dig-setting.auto-dig.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.keyword.dig-setting.auto-dig.yes',
                }),
                value: true,
              },
            ]}
          />
          <ProFormDigit
            name="max_count"
            label={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.max-count',
            })}
            extra={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.max-count.description',
            })}
            placeholder={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.max-count.placeholder',
            })}
          />
          <ProFormRadio.Group
            name="language"
            label={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.language',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.translate.zh-cn',
                }),
                value: 'zh-CN',
              },
              {
                label: intl.formatMessage({
                  id: 'content.translate.en',
                }),
                value: 'en',
              },
            ]}
          />
          <ProFormText
            label={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.title-exclude',
            })}
            fieldProps={{
              value: tmpInput.title_exclude || '',
              onChange: (e) => handleChangeTmpInput('title_exclude', e),
              onPressEnter: () => handleAddField('title_exclude'),
              suffix: (
                <a onClick={() => handleAddField('title_exclude')}>
                  <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                </a>
              ),
            }}
            extra={
              <div>
                <div className="text-muted">
                  <FormattedMessage id="plugin.keyword.dig-setting.title-exclude.description" />
                </div>
                <div className="tag-lists">
                  <Space size={[12, 12]} wrap>
                    {setting.title_exclude?.map((tag: any, index: number) => (
                      <span className="edit-tag" key={index}>
                        <span className="key">{tag}</span>
                        <span
                          className="close"
                          onClick={() => handleRemove('title_exclude', index)}
                        >
                          ×
                        </span>
                      </span>
                    ))}
                  </Space>
                </div>
              </div>
            }
          />
          <ProFormText
            label={intl.formatMessage({
              id: 'plugin.keyword.dig-setting.replace',
            })}
            extra={
              <div>
                <div className="text-muted">
                  <p>
                    <FormattedMessage id="plugin.keyword.dig-setting.replace.tips1" />
                  </p>
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
                  <div>
                    <span className="text-red">*</span>{' '}
                    <FormattedMessage id="plugin.aigenerate.replace.notice" />
                  </div>
                </div>
                <div className="tag-lists">
                  <Space size={[12, 12]} wrap>
                    {setting.content_replace?.map((tag: any, index: number) => (
                      <span className="edit-tag" key={index}>
                        <span className="key">{tag.from}</span>
                        <span className="divide">
                          <FormattedMessage id="plugin.aigenerate.replace.to" />
                        </span>
                        <span className="value">
                          {tag.to ||
                            intl.formatMessage({
                              id: 'plugin.aigenerate.empty',
                            })}
                        </span>
                        <span
                          className="close"
                          onClick={() => handleRemove('content_replace', index)}
                        >
                          ×
                        </span>
                      </span>
                    ))}
                  </Space>
                </div>
              </div>
            }
          >
            <Input.Group compact>
              <Input
                style={{ width: '40%' }}
                value={tmpInput.from || ''}
                onChange={(e) => handleChangeTmpInput('from', e)}
                onPressEnter={() => handleAddField('content_replace')}
              />
              <span className="input-divide">
                <FormattedMessage id="plugin.aigenerate.replace.to" />
              </span>
              <Input
                style={{ width: '50%' }}
                value={tmpInput.to || ''}
                onChange={(e) => handleChangeTmpInput('to', e)}
                onPressEnter={() => handleAddField('content_replace')}
                suffix={
                  <a onClick={() => handleAddField('content_replace')}>
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                }
              />
            </Input.Group>
          </ProFormText>
        </ModalForm>
      )}
    </>
  );
};

export default KeywordSetting;
