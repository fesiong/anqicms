import MarkdownEditor from '@/components/markdown';
import {
  pluginGetSendmailTemplateDetail,
  pluginSaveSendmailTemplate,
  pluginSendmailTemplatePreview,
} from '@/services/plugin/sendmail';
import {
  ModalForm,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Alert, Button, Card, Col, Row, message } from 'antd';
import React, { useEffect, useState } from 'react';
import '../index.less';

export type EmailTemplateFormProps = {
  open: boolean;
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: any) => void;
  template: any;
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

const EmailTemplateForm: React.FC<EmailTemplateFormProps> = (props) => {
  const intl = useIntl();
  const [template, setTemplate] = useState<any>(props.template || {});
  const [codeValue, setCodeValue] = useState<string>('');
  const [subjectValue, setSubjectValue] = useState<string>('');
  const [preview, setPreview] = useState<boolean>(false);
  const [previewData, setPreviewData] = useState<any>({});

  useEffect(() => {
    pluginGetSendmailTemplateDetail({ key: props.template.key }).then((res) => {
      setTemplate(res.data || {});
      setCodeValue(res.data?.content || '');
      setSubjectValue(res.data?.subject || '');
    });
  }, [props.template]);

  const debouncedPreview = useDebounce(() => {
    pluginSendmailTemplatePreview({
      ...template,
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

  const editorDidMount = () => {
    //...
  };

  const handleSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    const postData = { ...template, ...values };
    postData.content = codeValue;
    pluginSaveSendmailTemplate(postData)
      .then((res) => {
        message.info(res.msg);
        props.onSubmit(values);
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <ModalForm
      width={preview ? 1300 : 1100}
      title={props.template.name}
      open={props.open}
      initialValues={props.template}
      layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        handleSubmit(values);
      }}
    >
      <Alert
        className="mb-normal mt-normal"
        message={props.template.description}
        type="info"
      />
      <ProFormText
        name="key"
        readonly
        label={intl.formatMessage({ id: 'plugin.sendmail.template.key' })}
      />
      <ProFormRadio.Group
        name="open"
        label={intl.formatMessage({ id: 'plugin.sendmail.template.status' })}
        options={[
          {
            label: intl.formatMessage({
              id: 'plugin.sendmail.template.status.enable',
            }),
            value: true,
          },
          {
            label: intl.formatMessage({
              id: 'plugin.sendmail.template.status.disable',
            }),
            value: false,
          },
        ]}
      />
      {!props.template.readonly && (
        <Row gutter={16}>
          <Col span={preview ? 12 : 24}>
            <ProFormText
              name="subject"
              label={intl.formatMessage({
                id: 'plugin.sendmail.template.subject',
              })}
              fieldProps={{
                onChange: (e) => onChangeSubject(e.target.value),
              }}
            />
            <ProFormText
              layout="horizontal"
              label={intl.formatMessage({
                id: 'plugin.sendmail.template.content',
              })}
            >
              <div></div>
            </ProFormText>
            <MarkdownEditor
              className="mb-normal email-editor"
              setContent={(html) => onChangeCode(html)}
              content={codeValue}
              ref={null}
            />
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
      )}
    </ModalForm>
  );
};

export default EmailTemplateForm;
