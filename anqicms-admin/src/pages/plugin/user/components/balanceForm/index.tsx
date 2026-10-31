import { pluginAddUserBalance } from '@/services';
import {
  ModalForm,
  ProFormDigit,
  ProFormInstance,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Alert, message } from 'antd';
import React, { useEffect, useRef } from 'react';

export type BalanceFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  user: any;
};

const BalanceForm: React.FC<BalanceFormProps> = (props) => {
  const formRef = useRef<ProFormInstance>();
  const intl = useIntl();

  useEffect(() => {}, []);

  const onSubmit = async (values: any) => {
    if (values.amount === 0) {
      message.error(intl.formatMessage({ id: 'plugin.user.balance.error' }));
      return;
    }
    const res = await pluginAddUserBalance({
      user_id: props.user.id,
      amount: Number((values.amount * 100).toFixed(0)),
      remark: values.remark,
    });
    message.info(res.msg);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={600}
      title={
        props.user?.id
          ? intl.formatMessage({ id: 'plugin.user.edit' })
          : intl.formatMessage({ id: 'plugin.user.add' })
      }
      open={props.open}
      layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      formRef={formRef}
      initialValues={props.user}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <Alert message={intl.formatMessage({ id: 'plugin.user.balance.tips' })} />
      <ProFormText
        name="id"
        label={intl.formatMessage({ id: 'plugin.user.user-id' })}
        readonly
      />
      <ProFormText
        name="user_name"
        label={intl.formatMessage({ id: 'plugin.user.user-name' })}
        readonly
      />
      <ProFormDigit
        name="balance"
        label={intl.formatMessage({ id: 'plugin.user.balance' })}
        width="lg"
        transform={(value, namePath) => {
          return { [namePath]: value > 0 ? value / 100 : 0 };
        }}
        fieldProps={{
          precision: 2,
        }}
        readonly
      />
      <ProFormDigit
        name="amount"
        label={intl.formatMessage({ id: 'plugin.user.balance-add' })}
        width="lg"
        fieldProps={{
          precision: 2,
        }}
        readonly
      />
      <ProFormText
        name="remark"
        label={intl.formatMessage({ id: 'plugin.user.balance-remark' })}
        readonly
      />
    </ModalForm>
  );
};

export default BalanceForm;
