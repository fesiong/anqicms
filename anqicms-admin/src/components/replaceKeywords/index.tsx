import { getCollectorSetting, replaceCollectorArticle } from '@/services/collector';
import { PlusOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Input, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

export type ReplaceKeywordsProps = {
  onCancel: (flag?: boolean) => void;
  open: boolean;
};

const ReplaceKeywords: React.FC<ReplaceKeywordsProps> = (props) => {
  const [keywords, setKeywords] = useState<any[]>([]);
  const [inputVisible, setInputVisible] = useState<boolean>(false);
  const [fromValue, setFromValue] = useState<string>('');
  const [toValue, setToValue] = useState<string>('');
  const intl = useIntl();

  let replaced = false;

  const getKeywords = async () => {
    const res = await getCollectorSetting();
    let keywords = res.data?.content_replace || [];
    setKeywords(keywords);
  };

  useEffect(() => {
    getKeywords();
  }, []);

  const handleStartReplace = () => {
    if (replaced) {
      message.info(intl.formatMessage({ id: 'component.replace.doing.tips' }));
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'component.replace.start.confirm' }),
      content: intl.formatMessage({ id: 'component.replace.start.confirm.content' }),
      onOk: () => {
        replaced = true;
        let hide = message.loading(intl.formatMessage({ id: 'component.replace.doing' }));
        replaceCollectorArticle({
          content_replace: keywords,
          replace: true,
        })
          .then((res) => {
            message.info(res.msg);
          })
          .catch(() => {})
          .finally(() => {
            hide();
          });
      },
    });
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

  const onSubmit = () => {
    replaceCollectorArticle({
      content_replace: keywords,
      replace: false,
    })
      .then((res) => {
        message.info(res.msg);
        props.onCancel();
      })
      .catch(() => {});
  };

  return (
    <Modal
      width={700}
      title={intl.formatMessage({ id: 'setting.action.cancel' })}
      footer={
        <Space>
          <Button
            onClick={() => {
              props.onCancel();
            }}
          >
            <FormattedMessage id="component.material.use" />
          </Button>
          <Button
            onClick={() => {
              handleStartReplace();
            }}
          >
            <FormattedMessage id="component.replace.batch-replace" />
          </Button>
          <Button
            type="primary"
            onClick={() => {
              onSubmit();
            }}
          >
            <FormattedMessage id="setting.action.submit" />
          </Button>
        </Space>
      }
      open={props.open}
      onCancel={() => {
        props.onCancel();
      }}
    >
      <Alert
        message={
          <div>
            <p>
              <FormattedMessage id="plugin.aigenerate.replace.tips1" />
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
            <p>
              <span className="text-red">*</span>{' '}
              <FormattedMessage id="plugin.aigenerate.replace.notice" />
            </p>
          </div>
        }
        type="info"
      />
      <div className="tag-lists">
        <Space size={[12, 12]} wrap>
          {keywords.map((tag: any, index: number) => (
            <span className="edit-tag" key={index}>
              <span className="key">{tag.from}</span>
              <span className="divide">
                <FormattedMessage id="plugin.aigenerate.replace.to" />
              </span>
              <span className="value">
                {tag.to || intl.formatMessage({ id: 'plugin.aigenerate.empty' })}
              </span>
              <span
                className="close"
                onClick={() => {
                  handleRemove(index);
                }}
              >
                ×
              </span>
            </span>
          ))}
          {!inputVisible && (
            <Button className="site-tag-plus" onClick={showInput}>
              <PlusOutlined /> <FormattedMessage id="component.replace.add" />
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
    </Modal>
  );
};

export default ReplaceKeywords;
