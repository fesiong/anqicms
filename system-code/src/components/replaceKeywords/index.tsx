import React, { useEffect, useState } from 'react';
import { Alert, Button, Input, message, Modal, Space, Tag } from 'antd';
import './index.less';
import { getCollectorSetting, replaceCollectorArticle } from '@/services/collector';
import { PlusOutlined } from '@ant-design/icons';

export type ReplaceKeywordsProps = {
  onCancel: (flag?: boolean) => void;
  visible: boolean;
};

const ReplaceKeywords: React.FC<ReplaceKeywordsProps> = (props) => {
  const [keywords, setKeywords] = useState<any[]>([]);
  const [inputVisible, setInputVisible] = useState<boolean>(false);
  const [fromValue, setFromValue] = useState<string>('');
  const [toValue, setToValue] = useState<string>('');

  var replaced = false;

  useEffect(() => {
    getKeywords();
  }, []);

  const getKeywords = async () => {
    const res = await getCollectorSetting();
    let keywords = res.data.content_replace || [];
    setKeywords(keywords);
  };

  const handleStartReplace = () => {
    if (replaced) {
      message.info('批量替换操作正在执行中，无需再次点击执行');
    }
    Modal.confirm({
      title: '确定要执行批量替换文章关键词操作吗？',
      content: '该操作会根据你设置的需要替换的关键词，对所有的文章都执行一遍替换操作。',
      cancelText: '取消',
      okText: '确定',
      onOk: () => {
        replaced = true;
        let hide = message.loading('处理中');
        replaceCollectorArticle({
          content_replace: keywords,
          replace: true,
        })
          .then((res) => {
            message.info(res.msg);
          })
          .catch((err) => {})
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
    if (field == 'from') {
      setFromValue(e.target.value);
    } else if (field == 'to') {
      setToValue(e.target.value);
    }
  };

  const handleEditInputConfirm = () => {
    let tag: any = {
      from: fromValue,
      to: toValue,
    };
    keywords.push(tag);
    console.log(keywords);

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
      .catch((err) => {});
  };

  return (
    <Modal
      width={700}
      title="关键词替换管理"
      footer={
        <Space>
          <Button
            onClick={() => {
              props.onCancel();
            }}
          >
            取消
          </Button>
          <Button
            onClick={() => {
              handleStartReplace();
            }}
          >
            批量替换文章关键词
          </Button>
          <Button
            type="primary"
            onClick={() => {
              onSubmit();
            }}
          >
            保存
          </Button>
        </Space>
      }
      visible={props.visible}
      onCancel={() => {
        props.onCancel();
      }}
    >
      <Alert
        message={
          <div>
            <p>编辑需要替换的关键词对，会在发布文章的时候自动执行替换。</p>
            <p>
              替换规则支持正则表达式，如果你对正则表达式熟悉，并且通过普通文本无法达成替换需求的，可以尝试使用正则表达式规则来完成替换。
            </p>
            <p>
              正则表达式规则为：由 <Tag>{'{'}</Tag>开始，并以 <Tag>{'}'}</Tag>
              结束，中间书写规则代码，如{' '}
              <Tag>
                {'{'}[0-9]+{'}'}
              </Tag>{' '}
              代表匹配连续的数字。
            </p>
            <p>
              内置部分规则，可以快速使用，已内置的有：
              <Tag>
                {'{'}邮箱地址{'}'}
              </Tag>
              、
              <Tag>
                {'{'}日期{'}'}
              </Tag>
              、
              <Tag>
                {'{'}时间{'}'}
              </Tag>
              、
              <Tag>
                {'{'}电话号码{'}'}
              </Tag>
              、
              <Tag>
                {'{'}QQ号{'}'}
              </Tag>
              、
              <Tag>
                {'{'}微信号{'}'}
              </Tag>
              、
              <Tag>
                {'{'}网址{'}'}
              </Tag>
            </p>
            <p>
              <span className="text-red">*</span>{' '}
              注意：正则表达式规则书写不当很容易造成错误的替换效果，如微信号规则，会同时影响到邮箱地址、网址的完整性。请谨慎使用。
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
              <span className="divide">替换为</span>
              <span className="value">{tag.to || '空'}</span>
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
              <PlusOutlined /> 新增替换关键词
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
          <span className="input-divide">替换为</span>
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
            回车提交
          </Button>
        </Input.Group>
      )}
    </Modal>
  );
};

export default ReplaceKeywords;
