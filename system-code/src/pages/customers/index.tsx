import { Button, message, Input, Drawer, Modal, Space, Alert } from 'antd';
import React, { useState, useRef } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { customerChangeSales, getCustomers } from '@/services/customer';
import { ModalForm, ProFormDigit } from '@ant-design/pro-form';

const SystemCustomers: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [editingCustomer, setEditingCustomer] = useState<any>({});
  const [editingVisible, setEditingVisible] = useState<boolean>(false);

  const handleSubmit = (values: any) => {
    customerChangeSales({
      id: editingCustomer.id,
      invite_id: values.invite_id,
    }).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
      } else {
        message.success(res.msg);
        handleHideModal();
        actionRef?.current?.reload();
      }
    });
  };

  const handleHideModal = () => {
    setEditingCustomer({});
    setEditingVisible(false);
  };

  const handleShowEdit = (record: any) => {
    setEditingCustomer(record);
    setEditingVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: '序号',
      dataIndex: 'id',
    },
    {
      title: '用户名',
      dataIndex: 'account',
    },
    {
      title: '手机号',
      dataIndex: 'mobile',
    },
    {
      title: '联系人',
      dataIndex: 'liaison',
    },
    {
      title: '销售员',
      dataIndex: 'invite_name',
    },
    {
      title: '加入时间',
      dataIndex: 'created_at',
      render: (text) => moment(text).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      key: 'action',
      render: (text: any, record: any) => (
        <span>
          <Button
            type="link"
            onClick={() => {
              handleShowEdit(record);
            }}
          >
            调整绑定销售员
          </Button>
        </span>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="销售员列表"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        request={(params, sort) => {
          return getCustomers(params);
        }}
        columns={columns}
      />
      {editingVisible && (
        <ModalForm
          title={editingCustomer.account + '的销售员'}
          initialValues={editingCustomer}
          visible={editingVisible}
          onFinish={async (values) => {
            handleSubmit(values);
          }}
          modalProps={{
            onCancel: handleHideModal,
          }}
          width={500}
        >
          <Alert className="mb-normal" message="销售员的ID请在销售员列表中查找" />
          <ProFormDigit name="invite_id" label="销售员ID" extra="请填写数字" />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default SystemCustomers;
