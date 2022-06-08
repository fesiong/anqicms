import React from 'react';
import { ModalForm, ProFormDigit, ProFormText } from '@ant-design/pro-form';

import { pluginSaveAnchor } from '@/services/plugin/anchor';

export type AnchorFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  visible: boolean;
  editingAnchor: any;
};

const AnchorForm: React.FC<AnchorFormProps> = (props) => {
  const onSubmit = async (values: any) => {
    let editingAnchor = Object.assign(props.editingAnchor, values);
    let res = await pluginSaveAnchor(editingAnchor);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={props.editingAnchor?.id ? '编辑锚文本' : '添加锚文本'}
      initialValues={props.editingAnchor}
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
      <ProFormText name="title" label="锚文本名称" />
      <ProFormText
        name="link"
        label="锚文本链接"
        extra={'支持相对链接和绝对连接，如：/a/123.html 或 https://www.kandaoni.com/'}
      />
      <ProFormDigit
        name="weight"
        label="锚文本权重"
        extra={'请输入数字，0-9，数字越大，权重越高，高权重拥有优先替换权'}
      />
    </ModalForm>
  );
};

export default AnchorForm;
