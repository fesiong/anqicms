import {
  anqiAiGenerateStream,
  anqiAiGenerateStreamData,
  getAnqiInfo,
  getCollectCombineArticle,
} from '@/services';
import { ProForm, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Divider, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

export type AiGenerateProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: any) => Promise<void>;
  open: boolean;
  title: string;
  editor?: string;
};

let xhr: any = null;

const AiGenerate: React.FC<AiGenerateProps> = (props) => {
  const [anqiUser, setAnqiUser] = useState<any>({});
  const [aiTitle, setAiTitle] = useState<string>('');
  const [aiDemand, setAiDemand] = useState<string>('');
  const [aiContent, setAiContent] = useState<string>('');
  const [aiFinished, setAiFinished] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const intl = useIntl();

  // 临时的
  let tmpContent = '';

  useEffect(() => {
    // 获取AIremain
    getAnqiInfo().then((res) => {
      setAnqiUser(res.data || {});
    });
    setAiTitle(props.title || '');

    return () => {
      clearInterval(xhr);
    };
  }, []);

  const onChangeAiTitle = (e: any) => {
    setAiTitle(e.target.value);
  };

  const onChangeAiDemand = (e: any) => {
    setAiDemand(e.target.value);
  };

  const finishedGenerate = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'component.aigenerate.submit.title' }),
      content: intl.formatMessage({
        id: 'component.aigenerate.submit.content',
      }),
      onOk: () => {
        props.onSubmit({
          title: aiTitle,
          content: aiContent,
        });
      },
    });
  };

  const startCombine = () => {
    if (loading) {
      return;
    }
    if (aiTitle.length < 2) {
      message.error(
        intl.formatMessage({ id: 'component.aigenerate.title.required' }),
      );
      return;
    }
    setLoading(true);
    getCollectCombineArticle({
      title: aiTitle,
      demand: aiDemand,
    })
      .then((res) => {
        if (!res || res.code !== 0) {
          message.error(
            res?.msg ||
              intl.formatMessage({ id: 'component.aigenerate.error' }),
          );
          return;
        }
        setAiTitle(res.data.title);
        setAiContent(res.data.content);
        setAiFinished(true);
      })
      .catch(() => {})
      .finally(() => {
        setLoading(false);
      });
  };

  const getStreamData = (streamId: string) => {
    anqiAiGenerateStreamData({
      stream_id: streamId,
    }).then((res) => {
      if (res.code !== 0) {
        setLoading(false);
        clearInterval(xhr);
        Modal.error({
          title: res.msg,
        });
        return;
      }
      setAiFinished(true);
      if (res.data) {
        if (!tmpContent && props.editor !== 'markdown') {
          tmpContent += '<p>';
        }
        if (props.editor === 'markdown') {
          tmpContent += res.data;
        } else {
          tmpContent += res.data.replace(/\n+/g, '</p>\n<p>');
        }
        setAiContent(tmpContent);
      }

      if (res.msg === 'finished') {
        if (tmpContent && props.editor !== 'markdown') {
          tmpContent += '</p>';
          setAiContent(tmpContent);
        }
        setLoading(false);
        clearInterval(xhr);
      }
    });
  };

  const startGenerate = () => {
    if (loading) {
      return;
    }
    if (aiTitle.length < 2) {
      message.error(
        intl.formatMessage({ id: 'component.aigenerate.title.required' }),
      );
      return;
    }
    setLoading(true);
    anqiAiGenerateStream({
      title: aiTitle,
      demand: aiDemand,
    })
      .then(async (res) => {
        if (res.code !== 0) {
          setLoading(false);
          message.error(res.msg);
          return;
        }
        // 重置
        tmpContent = '';
        let streamId = res.data;
        xhr = setInterval(() => {
          getStreamData(streamId);
        }, 1000);
      })
      .catch(() => {})
      .finally(() => {});
  };

  return (
    <Modal
      width={800}
      title={<FormattedMessage id="component.aigenerate.name" />}
      open={props.open}
      onCancel={() => {
        props.onCancel();
      }}
      footer={null}
    >
      <div className="mb-normal">
        <div className="extra-text">
          <Space>
            <span>
              <FormattedMessage id="component.right-content.integral" />
              {anqiUser.integral}
            </span>
            <span>
              <FormattedMessage id="component.right-content.free-token" />
              {anqiUser.free_token}
            </span>
            <span>
              <FormattedMessage id="component.right-content.total-token" />
              {anqiUser.total_token}
            </span>
            <span>
              <FormattedMessage id="component.right-content.un-pay-token" />
              {anqiUser.un_pay_token}
            </span>
            {anqiUser.is_owe_fee === 1 && (
              <Tag color="red">
                <FormattedMessage id="component.right-content.is-owe-fee" />
              </Tag>
            )}
          </Space>
          <div>
            <FormattedMessage id="component.right-content.total-token.description" />
          </div>
        </div>
      </div>
      <ProForm layout="horizontal" submitter={false}>
        <ProFormText
          label={intl.formatMessage({ id: 'component.aigenerate.title' })}
          name="title"
          placeholder={intl.formatMessage({
            id: 'component.aigenerate.title.placeholder',
          })}
          fieldProps={{
            value: aiTitle,
            onChange: onChangeAiTitle,
          }}
        />
        <ProFormText
          label={intl.formatMessage({ id: 'component.aigenerate.demand' })}
          name="demand"
          placeholder={intl.formatMessage({
            id: 'component.aigenerate.demand.placeholder',
          })}
          fieldProps={{
            maxLength: 150,
            value: aiDemand,
            onChange: onChangeAiDemand,
          }}
        />
        <div className="generate-btn mb-normal">
          <Space size={20}>
            <Button
              onClick={startGenerate}
              loading={loading}
              disabled={aiFinished}
            >
              <FormattedMessage id="component.aigenerate.btn.start-generate" />
            </Button>
            <Button
              onClick={startCombine}
              loading={loading}
              disabled={aiFinished}
            >
              <FormattedMessage id="component.aigenerate.btn.start-combine" />
            </Button>
            {aiFinished && (
              <>
                <Button onClick={finishedGenerate} loading={loading}>
                  <FormattedMessage id="component.aigenerate.btn.finish" />
                </Button>
                <Button onClick={() => props.onCancel()} loading={loading}>
                  <FormattedMessage id="component.aigenerate.btn.abundant" />
                </Button>
              </>
            )}
          </Space>
        </div>
        <Divider />
        <div
          className="article-detail"
          dangerouslySetInnerHTML={{
            __html: aiContent
              ? props.editor === 'markdown'
                ? aiContent.replace(/\n+/g, '</p>\n<p>')
                : aiContent
              : intl.formatMessage({
                  id: 'component.aigenerate.content.default',
                }),
          }}
        ></div>
      </ProForm>
    </Modal>
  );
};

export default AiGenerate;
