import React from 'react';
import {
  ModalForm,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';

import moment from 'moment';
import { pluginSaveComment } from '@/services/plugin/comment';

export type CommentFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  visible: boolean;
  editingComment: any;
};

const CommentForm: React.FC<CommentFormProps> = (props) => {
  const onSubmit = async (values: any) => {
    let editingLink = Object.assign(props.editingComment, values);
    let res = await pluginSaveComment(editingLink);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={600}
      title={props.editingComment?.id ? '编辑评论' : '添加评论'}
      initialValues={props.editingComment}
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
      <ProFormText name="id" label="ID" disabled />
      <ProFormRadio.Group
        disabled
        name="item_type"
        label="类型"
        options={[
          { value: 'article', label: '文章' },
          { value: 'product', label: '产品' },
        ]}
      />
      <ProFormText name="item_title" label="名称" disabled />
      <ProFormText
        fieldProps={{
          value: moment(props.editingComment.created_time * 1000).format('YYYY-MM-DD HH:mm:ss'),
        }}
        label="评论时间"
        disabled
      />
      <ProFormText name="ip" label="评论IP" />
      {props.editingComment.parent_id > 0 && props.editingComment.parent && (
        <ProFormTextArea name={['parent', 'content']} label="上级评论" />
      )}
      {props.editingComment.user_id > 0 && <ProFormText name="user_id" label="用户ID" disabled />}
      <ProFormText name="user_name" label="用户名" />
      <ProFormTextArea name="content" label="评论内容" />
    </ModalForm>
  );
};

export default CommentForm;
