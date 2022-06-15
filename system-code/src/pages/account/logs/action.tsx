import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { getAdminActionLogs, getAdminLoginLogs } from '@/services';

const PluginSendmail: React.FC = () => {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<any>[] = [
    {
      title: '时间',
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'IP',
      dataIndex: 'ip',
    },
    {
      title: '管理员',
      dataIndex: 'user_name',
    },
    {
      title: '操作内容',
      dataIndex: 'log',
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="后台操作记录"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params, sort) => {
          return getAdminActionLogs(params);
        }}
        columns={columns}
      />
    </PageContainer>
  );
};

export default PluginSendmail;
