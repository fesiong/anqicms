import { pluginSaveAnchor } from '@/services/plugin/anchor';
import { ModalForm, ProFormDigit, ProFormText } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import React from 'react';

export type AnchorFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingAnchor: any;
};

const AnchorForm: React.FC<AnchorFormProps> = (props) => {
  const intl = useIntl();

  const onSubmit = async (values: any) => {
    let editingAnchor = Object.assign(props.editingAnchor, values);
    await pluginSaveAnchor(editingAnchor);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={
        props.editingAnchor?.id
          ? intl.formatMessage({ id: 'plugin.anchor.edit' })
          : intl.formatMessage({ id: 'plugin.anchor.new' })
      }
      initialValues={props.editingAnchor}
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
      <ProFormText name="title" label={intl.formatMessage({ id: 'plugin.anchor.title' })} />
      <ProFormText
        name="link"
        label={intl.formatMessage({ id: 'plugin.anchor.link' })}
        extra={intl.formatMessage({ id: 'plugin.anchor.link.description' })}
      />
      <ProFormDigit
        name="weight"
        label={intl.formatMessage({ id: 'plugin.anchor.weight' })}
        extra={intl.formatMessage({ id: 'plugin.anchor.weight.description' })}
      />
    </ModalForm>
  );
};

export default AnchorForm;
