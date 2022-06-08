import React from 'react';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';

import { Collapse } from 'antd';
import { pluginSaveLink } from '@/services/plugin/link';

export type LinkFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  visible: boolean;
  editingLink: any;
};

const LinkForm: React.FC<LinkFormProps> = (props) => {
  const onSubmit = async (values: any) => {
    let editingLink = Object.assign(props.editingLink, values);
    let res = await pluginSaveLink(editingLink);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={props.editingLink?.id ? '编辑友情链接' : '添加友情链接'}
      initialValues={props.editingLink}
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
      <ProFormText name="title" label="对方关键词" />
      <ProFormText name="link" label="对方链接" extra={'如：https://www.kandaoni.com/'} />
      <ProFormRadio.Group
        name="nofollow"
        label="NOFOLLOW"
        options={[
          { value: 0, label: '不添加' },
          { value: 1, label: '添加' },
        ]}
        extra="是否添加nofollow标签"
      />
      <ProFormDigit name="sort" label="显示顺序" extra={'值越小，排序越靠前，默认99'} />
      <Collapse>
        <Collapse.Panel header="更多选项" key="1">
          <ProFormText name="back_link" label="对方反链页" extra={'对方放置本站链接的页面URL'} />
          <ProFormText name="my_title" label="我的关键词" extra={'我放在对方页面的关键词'} />
          <ProFormText name="my_link" label="我的链接" extra={'我放在对方页面的链接'} />
          <ProFormText
            name="contact"
            label="对方联系方式"
            extra={'填写QQ、微信、电话等，方便后续联系'}
          />
          <ProFormTextArea name="remark" label="备注信息" />
        </Collapse.Panel>
      </Collapse>
    </ModalForm>
  );
};

export default LinkForm;
