import {
  anqiAiImageConfirm,
  anqiAiImageGenerate,
  anqiAiImageHistories,
  getAnqiInfo,
} from '@/services';
import {
  ProColumns,
  ProForm,
  ProFormDigit,
  ProFormFieldSet,
  ProFormInstance,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Image, InputRef, Modal, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import './index.less';

export type AiImageGenerateProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: () => void;
  open: boolean;
  attach?: any;
  intl?: any;
};

const AiImageGenerate: React.FC<AiImageGenerateProps> = (props) => {
  const formRef = useRef<ProFormInstance>();
  const [anqiUser, setAnqiUser] = useState<any>({});
  const [aiResult, setAiResult] = useState<any>({});
  const [aiFinished, setAiFinished] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const [historyVisible, setHistoryVisible] = useState<boolean>(false);

  const inputRef = useRef<InputRef | null>(null);

  useEffect(() => {
    // 获取AIremain
    getAnqiInfo().then((res) => {
      setAnqiUser(res.data || {});
    });
  }, []);

  const startGenerate = () => {
    if (loading) {
      return;
    }
    const values = formRef.current?.getFieldsValue();
    if (values.prompt.length < 2) {
      message.error(
        props.intl?.formatMessage({
          id: 'component.aiimage.prompt.placeholder',
        }),
      );
      return;
    }
    let size = '';
    if (values.width > 1024 || values.height > 1024) {
      size = '1024x1024';
    } else if (values.width > 0 && values.height > 0) {
      size = values.width + 'x' + values.height;
    } else if (props.attach?.width > 0) {
      let width = values.width > 0 ? values.width : props.attach.width;
      let height = values.height > 0 ? values.height : props.attach.height;
      if (width > 1024) {
        width = 1024;
      }
      if (height > 1024) {
        height = 1024;
      }
      size = width + 'x' + height;
    }
    setLoading(true);
    const hide = message.loading(
      {
        content: props.intl?.formatMessage({
          id: 'component.aiimage.loading',
        }),
      },
      0,
    );
    setAiResult({});
    anqiAiImageGenerate({
      prompt: values.prompt,
      type: props.attach?.file_path ? 2 : 0,
      image: props.attach?.file_path,
      size: size,
    })
      .then(async (res) => {
        setAiFinished(true);
        setLoading(false);
        if (res.code !== 0) {
          Modal.error({
            title: res.msg,
          });
          return;
        }
        setAiResult(res.data);
      })
      .catch(() => {})
      .finally(() => {
        hide();
        setLoading(false);
      });
  };

  const handleSaveToAttachment = (record: any) => {
    Modal.confirm({
      zIndex: 9999,
      title: props.intl?.formatMessage({
        id: 'component.aiimage.title',
      }),
      icon: null,
      width: 600,
      content: (
        <div>
          <ProFormText
            placeholder={props.intl?.formatMessage({
              id: 'component.aiimage.title.placeholder',
            })}
            fieldProps={{
              ref: inputRef,
            }}
          />
        </div>
      ),
      onOk: async () => {
        const inputValue = inputRef.current?.input?.value;
        console.log(inputValue);
        anqiAiImageConfirm({
          action: 1,
          url: record.result,
          title: inputValue,
        }).then(() => {
          message.success(
            props.intl?.formatMessage({ id: 'component.aiimage.save.success' }),
          );
          props.onSubmit();
        });
      },
    });
  };

  const finishedGenerate = () => {
    handleSaveToAttachment(aiResult);
  };

  const finishedGenerateAndReplace = () => {
    handleSaveToAttachment({
      ...aiResult,
      replace: true,
      replace_id: props.attach?.id,
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: props.intl?.formatMessage({ id: 'component.aiimage.time' }),
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: props.intl?.formatMessage({ id: 'component.aiimage.prompt' }),
      width: 160,
      dataIndex: 'prompt',
    },
    {
      title: props.intl?.formatMessage({ id: 'component.aiimage.result' }),
      dataIndex: 'result',
      render: (text, record) => <Image src={record.result} width={100} />,
    },
    {
      title: props.intl?.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 150,
      render: (_, record) => (
        <Space size={20}>
          <a
            key="save"
            onClick={() => {
              handleSaveToAttachment(record);
            }}
          >
            {props.intl?.formatMessage({ id: 'component.aiimage.save' })}
          </a>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Modal
        width={800}
        title={props.intl?.formatMessage({ id: 'component.aiimage.generate' })}
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
                {props.intl?.formatMessage({
                  id: 'component.right-content.integral',
                })}
                {anqiUser.integral}
              </span>
              <span>
                {props.intl?.formatMessage({
                  id: 'component.right-content.free-token',
                })}
                {anqiUser.free_token}
              </span>
              <span>
                {props.intl?.formatMessage({
                  id: 'component.right-content.total-token',
                })}
                {anqiUser.total_token}
              </span>
              <span>
                {props.intl?.formatMessage({
                  id: 'component.right-content.un-pay-token',
                })}
                {anqiUser.un_pay_token}
              </span>
              {anqiUser.is_owe_fee === 1 && (
                <Tag color="red">
                  {props.intl?.formatMessage({
                    id: 'component.right-content.is-owe-fee',
                  })}
                </Tag>
              )}
            </Space>
            <div>
              {props.intl?.formatMessage({
                id: 'component.right-content.total-token.description',
              })}
            </div>
          </div>
        </div>
        <ProForm layout="horizontal" formRef={formRef} submitter={false}>
          {props.attach?.file_path && (
            <ProFormText
              label={props.intl?.formatMessage({
                id: 'component.aiimage.image',
              })}
            >
              <Image src={props.attach?.file_path} width={160} />
            </ProFormText>
          )}
          <ProFormTextArea
            label={props.intl?.formatMessage({
              id: 'component.aiimage.prompt',
            })}
            name="prompt"
            fieldProps={{
              maxLength: 1000,
            }}
          />
          <ProFormFieldSet
            label={props.intl?.formatMessage({
              id: 'component.aiimage.size',
            })}
          >
            <ProFormDigit
              name="width"
              max={1024}
              placeholder={'1024'}
              fieldProps={{
                defaultValue: props.attach?.width,
              }}
            />
            <div>x</div>
            <ProFormDigit
              name="height"
              max={1024}
              placeholder={'1024'}
              fieldProps={{
                defaultValue: props.attach?.height,
              }}
            />
          </ProFormFieldSet>
          {aiFinished && (
            <>
              <ProFormText
                label={props.intl?.formatMessage({
                  id: 'component.aiimage.result',
                })}
              >
                <Image src={aiResult.result} width={200} />
              </ProFormText>
            </>
          )}
          <div className="generate-btn mb-normal">
            <Space size={20}>
              <Button
                onClick={() => setHistoryVisible(true)}
                loading={loading}
                disabled={loading}
              >
                {props.intl?.formatMessage({
                  id: 'component.aiimage.history',
                })}
              </Button>
              <Button
                onClick={startGenerate}
                loading={loading}
                disabled={aiFinished}
              >
                {props.intl?.formatMessage({
                  id: 'component.aiimage.btn.generate',
                })}
              </Button>
              {aiFinished && (
                <>
                  {props.attach?.id ? (
                    <>
                      <Button
                        onClick={finishedGenerateAndReplace}
                        loading={loading}
                      >
                        {props.intl?.formatMessage({
                          id: 'component.aiimage.btn.replace',
                        })}
                      </Button>
                      <Button onClick={finishedGenerate} loading={loading}>
                        {props.intl?.formatMessage({
                          id: 'component.aiimage.btn.newimage',
                        })}
                      </Button>
                    </>
                  ) : (
                    <Button onClick={finishedGenerate} loading={loading}>
                      {props.intl?.formatMessage({
                        id: 'component.aigenerate.btn.finish',
                      })}
                    </Button>
                  )}
                  <Button onClick={() => props.onCancel()} loading={loading}>
                    {props.intl?.formatMessage({
                      id: 'component.aigenerate.btn.abundant',
                    })}
                  </Button>
                </>
              )}
            </Space>
          </div>
        </ProForm>
      </Modal>
      {historyVisible && (
        <Modal
          title={props.intl?.formatMessage({ id: 'component.aiimage.history' })}
          width={900}
          open={historyVisible}
          onCancel={() => {
            setHistoryVisible(false);
          }}
          onOk={() => {
            setHistoryVisible(false);
          }}
        >
          <ProTable<any>
            rowKey="id"
            search={false}
            toolBarRender={false}
            request={(params) => {
              return anqiAiImageHistories(params);
            }}
            columnsState={{
              persistenceKey: 'push-log-table',
              persistenceType: 'localStorage',
            }}
            columns={columns}
            pagination={{
              showSizeChanger: true,
              showQuickJumper: true,
            }}
          />
        </Modal>
      )}
    </>
  );
};

export default AiImageGenerate;
