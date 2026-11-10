import {
  pluginGetSendmailSetting,
  pluginTestSendmail,
} from '@/services/plugin/sendmail';
import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import React, { useEffect, useState } from 'react';

import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Image, message } from 'antd';
import dayjs from 'dayjs';

export type GuestbookFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingGuestbook: any;
};

const GuestbookForm: React.FC<GuestbookFormProps> = (props) => {
  const [sendMailSetting, setSendMailSetting] = useState<any>({});
  const [replyVisible, setReplyVisible] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const intl = useIntl();

  const getSendMailSetting = async () => {
    let res = await pluginGetSendmailSetting();
    setSendMailSetting(res.data || {});
  };

  useEffect(() => {
    getSendMailSetting();
  }, []);

  const replyEmail = () => {
    if (!sendMailSetting.recipient && !sendMailSetting.account) {
      message.error(
        intl.formatMessage({ id: 'plugin.guestbook.reply.required' }),
      );
      return;
    }
    setReplyVisible(true);
  };

  const onSubmitReply = async (values: any) => {
    if (loading) {
      return;
    }
    setLoading(true);
    if (!values.message || !values.subject) {
      setLoading(false);
      message.error(
        intl.formatMessage({ id: 'plugin.guestbook.replysubmit.required' }),
      );
      return;
    }
    let res = await pluginTestSendmail({
      subject: values.subject,
      message: values.message,
      recipient: props.editingGuestbook.contact,
    });
    setLoading(false);
    if (res.code === 0) {
      message.success(
        intl.formatMessage({ id: 'plugin.guestbook.replysubmit.success' }),
      );
      setReplyVisible(false);
    } else {
      message.error(res.msg);
    }
  };

  return (
    <>
      <ModalForm
        width={600}
        title={intl.formatMessage({ id: 'plugin.guestbook.view' })}
        initialValues={props.editingGuestbook}
        open={props.open}
        layout="horizontal"
        onOpenChange={(flag) => {
          if (!flag) {
            props.onCancel(flag);
          }
        }}
        onFinish={async () => {
          props.onCancel(false);
        }}
      >
        <ProFormText name="id" label="ID" readonly />
        <ProFormText
          name="user_name"
          label={intl.formatMessage({ id: 'plugin.guestbook.user-name' })}
          readonly
        />
        <ProFormText
          name="contact"
          label={intl.formatMessage({ id: 'plugin.guestbook.contact' })}
          readonly
          extra={
            props.editingGuestbook.contact?.indexOf('@') !== -1 && (
              <Button onClick={replyEmail}>
                <FormattedMessage id="plugin.guestbook.reply" />
              </Button>
            )
          }
        />
        <ProFormTextArea
          name="content"
          label={intl.formatMessage({ id: 'plugin.guestbook.content' })}
          readonly
        />
        {Object.keys(props.editingGuestbook.extra_data || {}).map(
          (key: string, index: number) => (
            <ProFormText
              key={index}
              name={key}
              initialValue={props.editingGuestbook.extra_data[key]}
              label={key}
              readonly
              extra={
                props.editingGuestbook.extra_data[key]?.indexOf('http') !==
                  -1 && (
                  <a
                    href={props.editingGuestbook.extra_data[key]}
                    target={'_blank'}
                    rel="noopener noreferrer"
                  >
                    {props.editingGuestbook.extra_data[key].indexOf('.jpg') !==
                      -1 ||
                    props.editingGuestbook.extra_data[key].indexOf('.jpeg') !==
                      -1 ||
                    props.editingGuestbook.extra_data[key].indexOf('.png') !==
                      -1 ||
                    props.editingGuestbook.extra_data[key].indexOf('.webp') !==
                      -1 ||
                    props.editingGuestbook.extra_data[key].indexOf('.gif') !==
                      -1 ? (
                      <Image
                        width={200}
                        src={props.editingGuestbook.extra_data[key]}
                      />
                    ) : (
                      intl.formatMessage({
                        id: 'plugin.guestbook.click-preview',
                      })
                    )}
                  </a>
                )
              }
            />
          ),
        )}
        <ProFormText name="ip" label="IP" readonly />
        <ProFormText
          name="refer"
          label={intl.formatMessage({ id: 'plugin.guestbook.refer' })}
          readonly
        />
        <ProFormText
          fieldProps={{
            value: dayjs(props.editingGuestbook.created_time * 1000).format(
              'YYYY-MM-DD HH:mm:ss',
            ),
          }}
          label={intl.formatMessage({ id: 'plugin.guestbook.create-time' })}
          readonly
        />
      </ModalForm>
      <ModalForm
        width={550}
        title={intl.formatMessage({ id: 'plugin.guestbook.reply' })}
        open={replyVisible}
        layout="horizontal"
        onOpenChange={(flag) => {
          setReplyVisible(flag);
        }}
        onFinish={onSubmitReply}
      >
        <ProFormText
          name="subject"
          label={intl.formatMessage({ id: 'plugin.guestbook.reply.subject' })}
        />
        <ProFormTextArea
          name="message"
          label={intl.formatMessage({ id: 'plugin.guestbook.reply.message' })}
        />
      </ModalForm>
    </>
  );
};

export default GuestbookForm;
