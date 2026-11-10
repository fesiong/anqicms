import MarkdownEditor from '@/components/markdown';
import { getSettingContent } from '@/services';
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
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

export type ModuleFormProps = {
  onCancel: (flag?: boolean) => void;
  open: boolean;
};

const SubscriptionSendMail: React.FC<ModuleFormProps> = (props) => {
  const [sendType, setSendType] = useState<string>('');
  const [content, setContent] = useState<string>('');
  const [contentSetting, setContentSetting] = useState<any>({});
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
      content: content,
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

  return (
    <>
      <ModalForm
        width={1000}
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
        <ProFormText
          name="subject"
          required
          label={intl.formatMessage({
            id: 'plugin.subscription.send-mail.subject',
          })}
        />
        <ProFormText
          label={intl.formatMessage({
            id: 'plugin.subscription.send-mail.content',
          })}
        >
          <div style={{ height: 320 }}>
            <MarkdownEditor
              key="content"
              className="mb-normal"
              setContent={async (html) => setContent(html)}
              content={content}
              ref={null}
            />
          </div>
        </ProFormText>
      </ModalForm>
    </>
  );
};

export default SubscriptionSendMail;
