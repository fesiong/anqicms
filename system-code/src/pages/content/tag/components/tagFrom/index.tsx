import React from 'react';
import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';

import { saveTag } from '@/services/tag';

export type TagFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  type: number;
  visible: boolean;
  tag: any;
};

const TagForm: React.FC<TagFormProps> = (props) => {

  const onSubmit = async (values: any) => {
    let tag = Object.assign(props.tag, values);
    tag.type = props.type;
    let res = await saveTag(tag);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={props.tag?.id ? '编辑标签' : '添加标签'}
      initialValues={props.tag}
      visible={props.visible}
      layout="horizontal"
      onVisibleChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormText name="title" label="标签名称" />
      <ProFormText
        name="first_letter"
        label="索引字母"
        placeholder="默认会自动生成，无需填写"
        extra="注意：只能填写A-Z中任意一个"
      />
      <ProFormText
        name="url_token"
        label="自定义URL"
        placeholder="默认会自动生成，无需填写"
        extra="注意：自定义URL只能填写字母、数字和下划线，不能带空格"
      />
      <ProFormText
        name="seo_title"
        label="SEO标题"
        placeholder="默认为标签名称，无需填写"
        extra="注意：如果你希望页面的title标签的内容不是标签名称，可以通过SEO标题设置"
      />
      <ProFormText
        name="keywords"
        label="标签关键词"
        extra="你可以单独设置关键词"
      />
      <ProFormTextArea name="description" label="标签简介" />
    </ModalForm>
  );
};

export default TagForm;
