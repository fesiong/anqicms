import AttachmentSelect from '@/components/attachment';
import CollapseItem from '@/components/collaspeItem';
import { pluginSaveLink } from '@/services/plugin/link';
import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { message, Modal } from 'antd';
import React, { useEffect, useState } from 'react';

export type LinkFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingLink: any;
};

const LinkForm: React.FC<LinkFormProps> = (props) => {
  const intl = useIntl();
  const [logo, setLogo] = useState<string>('');

  useEffect(() => {
    setLogo(props.editingLink.logo || '');
  }, [props.editingLink]);
  const onSubmit = async (values: any) => {
    let editingLink = Object.assign(props.editingLink, values);
    editingLink.logo = logo;
    await pluginSaveLink(editingLink);

    props.onSubmit();
  };

  const handleSelectQrcode = (row: any) => {
    setLogo(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleRemoveQrcode = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        setLogo('');
      },
    });
  };

  return (
    <ModalForm
      width={800}
      title={
        props.editingLink?.id
          ? intl.formatMessage({ id: 'plugin.link.edit' })
          : intl.formatMessage({ id: 'plugin.link.add' })
      }
      initialValues={props.editingLink}
      open={props.open}
      //layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormText
        name="title"
        label={intl.formatMessage({ id: 'plugin.link.field.other-title' })}
      />
      <ProFormText
        name="link"
        label={intl.formatMessage({ id: 'plugin.link.field.other-link' })}
        extra={intl.formatMessage({
          id: 'plugin.link.field.other-link.description',
        })}
      />
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.jsonld.logo-image' })}
        width="lg"
      >
        <AttachmentSelect onSelect={handleSelectQrcode} open={false}>
          <div className="ant-upload-item">
            {logo ? (
              <>
                <img src={logo} style={{ width: '100%' }} />
                <a className="delete" onClick={handleRemoveQrcode}>
                  <FormattedMessage id="setting.system.delete" />
                </a>
              </>
            ) : (
              <div className="add">
                <PlusOutlined />
                <div style={{ marginTop: 8 }}>
                  <FormattedMessage id="setting.system.upload" />
                </div>
              </div>
            )}
          </div>
        </AttachmentSelect>
      </ProFormText>
      <ProFormRadio.Group
        name="nofollow"
        label="NOFOLLOW"
        options={[
          {
            value: 0,
            label: intl.formatMessage({ id: 'plugin.link.nofollow.no' }),
          },
          {
            value: 1,
            label: intl.formatMessage({ id: 'plugin.link.nofollow.yes' }),
          },
        ]}
        extra={intl.formatMessage({ id: 'plugin.link.nofollow.description' })}
      />
      <ProFormDigit
        name="sort"
        label={intl.formatMessage({ id: 'content.category.sort' })}
        extra={intl.formatMessage({ id: 'content.category.sort.description' })}
      />
      <CollapseItem
        header={intl.formatMessage({ id: 'plugin.link.more' })}
        showArrow
        key="1"
      >
        <ProFormText
          name="back_link"
          label={intl.formatMessage({ id: 'plugin.link.field.back-link' })}
          extra={intl.formatMessage({
            id: 'plugin.link.field.back-link.description',
          })}
        />
        <ProFormText
          name="my_title"
          label={intl.formatMessage({ id: 'plugin.link.field.self-title' })}
          extra={intl.formatMessage({
            id: 'plugin.link.field.self-title.description',
          })}
        />
        <ProFormText
          name="my_link"
          label={intl.formatMessage({ id: 'plugin.link.field.self-link' })}
          extra={intl.formatMessage({
            id: 'plugin.link.field.self-link.description',
          })}
        />
        <ProFormText
          name="contact"
          label={intl.formatMessage({ id: 'plugin.link.field.contact' })}
          extra={intl.formatMessage({
            id: 'plugin.link.field.contact.description',
          })}
        />
        <ProFormTextArea
          name="remark"
          label={intl.formatMessage({ id: 'plugin.link.field.remark' })}
        />
      </CollapseItem>
    </ModalForm>
  );
};

export default LinkForm;
