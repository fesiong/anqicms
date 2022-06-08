import React from 'react';
import {
  ModalForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';
import { Tag } from 'antd';
import { pluginSaveRedirect } from '@/services/plugin/redirect';

export type RedirectFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  visible: boolean;
  editingRedirect: any;
};

const RedirectForm: React.FC<RedirectFormProps> = (props) => {
  const onSubmit = async (values: any) => {
    let editingRedirect = Object.assign(props.editingRedirect, values);
    let res = await pluginSaveRedirect(editingRedirect);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={props.editingRedirect?.id ? '编辑链接' : '添加链接'}
      initialValues={props.editingRedirect}
      visible={props.visible}
      //layout="horizontal"
      onVisibleChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormText name="from_url" label="源链接" extra={<div className='mt-normal'>可以是绝对地址<Tag>http(https)</Tag>开头，或相对地址<Tag>/</Tag>开头的链接</div>} />
      <ProFormText name="to_url" label="跳转链接" extra={<div className='mt-normal'>可以是绝对地址<Tag>http(https)</Tag>开头，或相对地址<Tag>/</Tag>开头的链接</div>} />
    </ModalForm>
  );
};

export default RedirectForm;
