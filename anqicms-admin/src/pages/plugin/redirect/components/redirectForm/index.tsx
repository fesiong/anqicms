import { pluginSaveRedirect } from '@/services/plugin/redirect';
import { ModalForm, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import React from 'react';

export type RedirectFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingRedirect: any;
};

const RedirectForm: React.FC<RedirectFormProps> = (props) => {
  const intl = useIntl();
  const onSubmit = async (values: any) => {
    let editingRedirect = Object.assign(props.editingRedirect, values);
    await pluginSaveRedirect(editingRedirect);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={props.editingRedirect?.id ? intl.formatMessage({ id: 'plugin.redirect.edit' }) : intl.formatMessage({ id: 'plugin.redirect.add' })}
      initialValues={props.editingRedirect}
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
        name="from_url"
        label={intl.formatMessage({ id: 'plugin.redirect.from-url' })}
        extra={
          <div className="mt-normal">
            <FormattedMessage id="plugin.redirect.from-url.description" />
          </div>
        }
      />
      <ProFormText
        name="to_url"
        label={intl.formatMessage({ id: 'plugin.redirect.to-url' })}
        extra={
          <div className="mt-normal">
            <FormattedMessage id="plugin.redirect.from-url.description" />
          </div>
        }
      />
    </ModalForm>
  );
};

export default RedirectForm;
