import { pluginSaveComment } from '@/services/plugin/comment';
import { ModalForm, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import React from 'react';

export type CommentFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingComment: any;
};

const CommentForm: React.FC<CommentFormProps> = (props) => {
  const intl = useIntl();

  const onSubmit = async (values: any) => {
    let editingLink = Object.assign(props.editingComment, values);
    await pluginSaveComment(editingLink);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={600}
      title={
        props.editingComment?.id
          ? intl.formatMessage({ id: 'plugin.comment.edit' })
          : intl.formatMessage({ id: 'plugin.comment.new' })
      }
      initialValues={props.editingComment}
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
      <ProFormText name="id" label="ID" disabled />
      <ProFormText
        name="item_title"
        label={intl.formatMessage({ id: 'plugin.comment.item-title' })}
        disabled
      />
      <ProFormText
        fieldProps={{
          value: dayjs(props.editingComment.created_time * 1000).format('YYYY-MM-DD HH:mm:ss'),
        }}
        label={intl.formatMessage({ id: 'plugin.comment.time' })}
        disabled
      />
      <ProFormText name="ip" label={intl.formatMessage({ id: 'plugin.comment.ip' })} />
      {props.editingComment.parent_id > 0 && props.editingComment.parent && (
        <ProFormTextArea
          name={['parent', 'content']}
          label={intl.formatMessage({ id: 'plugin.comment.parent' })}
        />
      )}
      {props.editingComment.user_id > 0 && (
        <ProFormText
          name="user_id"
          label={intl.formatMessage({ id: 'plugin.comment.user-id' })}
          disabled
        />
      )}
      <ProFormText
        name="user_name"
        label={intl.formatMessage({ id: 'plugin.comment.user-name' })}
      />
      <ProFormTextArea
        name="content"
        label={intl.formatMessage({ id: 'plugin.comment.content' })}
      />
    </ModalForm>
  );
};

export default CommentForm;
