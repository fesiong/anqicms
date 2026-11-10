import MarkdownEditor from '@/components/markdown';
import { getSettingContent, pluginSendmailTemplatePreview } from '@/services';
import {
  pluginGetSubscriberCategories,
  pluginSubscriberSendMail,
} from '@/services/plugin/subscriber';
import {
  ModalForm,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Alert, Button, Card, Col, message, Row } from 'antd';
import React, { useEffect, useState } from 'react';
import '../index.less';

export type ModuleFormProps = {
  onCancel: (flag?: boolean) => void;
  open: boolean;
};

const useDebounce = (func: (...args: any[]) => void, delay: number) => {
  const timeoutRef = React.useRef<NodeJS.Timeout | null>(null);

  return (...args: any[]) => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
    timeoutRef.current = setTimeout(() => func.apply(null, args), delay);
  };
};

const SubscriptionSendMail: React.FC<ModuleFormProps> = (props) => {
  const [sendType, setSendType] = useState<string>('');
  const [codeValue, setCodeValue] = useState<string>('');
  const [subjectValue, setSubjectValue] = useState<string>('');
  const [contentSetting, setContentSetting] = useState<any>({});
  const [preview, setPreview] = useState<boolean>(false);
  const [previewData, setPreviewData] = useState<any>({});
  const [loading, setLoading] = useState<boolean>(false);
  const intl = useIntl();

  useEffect(() => {
    getSettingContent().then((res) => {
      setContentSetting(res.data || {});
    });
  }, []);

  const handleSendMail = async (values: any) => {
    if (loading) {
      return;
    }
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    setLoading(true);
    const postData = {
      ...values,
      content: codeValue,
    };
    pluginSubscriberSendMail(postData)
      .then((res) => {
        message.info(res.msg);
        props.onCancel(false);
      })
      .finally(() => {
        setLoading(false);
        hide();
      });
  };

  const debouncedPreview = useDebounce(() => {
    pluginSendmailTemplatePreview({
      subject: subjectValue,
      content: codeValue,
    }).then((res) => {
      setPreviewData(res.data || {});
    });
  }, 300);

  const getPreviewData = () => {
    debouncedPreview();
  };

  const onChangeSubject = (newValue: string) => {
    if (subjectValue === newValue) {
      return;
    }
    setSubjectValue(newValue);
    if (preview) {
      getPreviewData();
    }
  };

  const onChangeCode = async (newValue: string) => {
    if (codeValue === newValue) {
      return;
    }
    setCodeValue(newValue);
    if (preview) {
      getPreviewData();
    }
  };

  return (
    <>
      <ModalForm
        width={preview ? 1300 : 1100}
        title={intl.formatMessage({ id: 'plugin.subscription.send-mail' })}
        open={props.open}
        modalProps={{
          onCancel: () => {
            props.onCancel();
          },
        }}
        layout="vertical"
        onFinish={async (values) => {
          handleSendMail(values);
        }}
      >
        <ProFormRadio.Group
          name="type"
          required
          label={intl.formatMessage({
            id: 'plugin.subscription.send-mail.type',
          })}
          options={[
            {
              label: intl.formatMessage({
                id: 'plugin.subscription.send-mail.type.all',
              }),
              value: 'all',
            },
            {
              label: intl.formatMessage({
                id: 'plugin.subscription.send-mail.type.category',
              }),
              value: 'category',
            },
            {
              label: intl.formatMessage({
                id: 'plugin.subscription.send-mail.type.email',
              }),
              value: 'email',
            },
          ]}
          fieldProps={{
            onChange: (e) => {
              setSendType(e.target.value);
            },
          }}
        />
        {sendType === 'category' && (
          <ProFormSelect
            label={intl.formatMessage({
              id: 'plugin.subscription.category.name',
            })}
            name="category_id"
            width="lg"
            request={async () => {
              const res = await pluginGetSubscriberCategories();
              const data = [
                {
                  id: 0,
                  title: intl.formatMessage({
                    id: 'plugin.subscription.category.empty',
                  }),
                },
              ]
                .concat(res.data || [])
                .map((item) => ({ label: item.title, value: item.id }));

              return data;
            }}
          />
        )}
        {sendType === 'email' && (
          <ProFormSelect
            mode="tags"
            name="emails"
            label={intl.formatMessage({
              id: 'plugin.subscription.email',
            })}
            fieldProps={{
              tokenSeparators: [',', '，'],
              onSelect: (a: string) => {
                if (a.indexOf('@') < 0) {
                  message.error(
                    intl.formatMessage({
                      id: 'plugin.subscription.email.error',
                    }),
                  );
                }
                return true;
              },
            }}
          />
        )}
        <Row gutter={16}>
          <Col span={preview ? 12 : 24}>
            <ProFormText
              name="subject"
              required
              label={intl.formatMessage({
                id: 'plugin.subscription.send-mail.subject',
              })}
              fieldProps={{
                onChange: (e) => onChangeSubject(e.target.value),
              }}
            />
            <ProFormText
              label={intl.formatMessage({
                id: 'plugin.subscription.send-mail.content',
              })}
            >
              <div>
                <MarkdownEditor
                  key="content"
                  className="mb-normal email-editor"
                  setContent={async (html) => onChangeCode(html)}
                  content={codeValue}
                  ref={null}
                />
              </div>
            </ProFormText>
            <div>
              <Button
                className="mt-normal"
                onClick={() => {
                  setPreview(!preview);
                  getPreviewData();
                }}
              >
                {intl.formatMessage({
                  id: 'plugin.sendmail.template.preview',
                })}
              </Button>
            </div>
            <Alert
              className="template-tips mt-normal"
              message={
                <div
                  dangerouslySetInnerHTML={{
                    __html: intl.formatMessage({
                      id: 'plugin.sendmail.template.tips.html',
                    }),
                  }}
                ></div>
              }
              type="info"
            />
          </Col>
          {preview && (
            <Col span={12}>
              <Card
                className="template-preview mt-normal"
                title={previewData.subject || 'Subject Preview'}
              >
                <div
                  dangerouslySetInnerHTML={{
                    __html: previewData.content || 'Content Preview',
                  }}
                ></div>
              </Card>
            </Col>
          )}
        </Row>
      </ModalForm>
    </>
  );
};

export default SubscriptionSendMail;
