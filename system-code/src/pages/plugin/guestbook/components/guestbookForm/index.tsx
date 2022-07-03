import React from 'react';
import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';

import moment from 'moment';
import { Image } from 'antd';

export type GuestbookFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  visible: boolean;
  editingGuestbook: any;
};

const GuestbookForm: React.FC<GuestbookFormProps> = (props) => {
  return (
    <ModalForm
      width={600}
      title={'查看留言'}
      initialValues={props.editingGuestbook}
      visible={props.visible}
      layout="horizontal"
      onVisibleChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        props.onCancel(false);
      }}
    >
      <ProFormText name="id" label="ID" readonly />
      <ProFormText name="user_name" label="用户名" readonly />
      <ProFormText name="contact" label="联系方式" readonly />
      <ProFormTextArea name="content" label="留言内容" readonly />
      {Object.keys(props.editingGuestbook.extra_data || {}).map((key: string, index: number) => (
        <ProFormText
          key={index}
          name={key}
          initialValue={props.editingGuestbook.extra_data[key]}
          label={key}
          readonly
          extra={props.editingGuestbook.extra_data[key]?.indexOf('http') !== -1 &&
          <Image width={200} src={props.editingGuestbook.extra_data[key]} />
          }
        />
      ))}
      <ProFormText name="ip" label="IP" readonly />
      <ProFormText name="refer" label="来源" readonly />
      <ProFormText
        fieldProps={{
          value: moment(props.editingGuestbook.created_time * 1000).format('YYYY-MM-DD HH:mm:ss'),
        }}
        label="评论时间"
        readonly
      />
    </ModalForm>
  );
};

export default GuestbookForm;
