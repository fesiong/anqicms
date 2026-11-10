import { pluginDeleteUserGroup, pluginGetUserGroups } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';
import UserGroupForm from './components/groupForm';

const PluginUserGroup: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentGroup, setCurrentGroup] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleEditGroup = async (record: any) => {
    setCurrentGroup(record);
    setEditVisible(true);
  };

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.group.delete.confirm' }),
      onOk: () => {
        pluginDeleteUserGroup(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.group.name' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.group.description' }),
      dataIndex: 'description',
    },
    {
      title: intl.formatMessage({ id: 'plugin.group.level' }),
      dataIndex: 'level',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditGroup(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            onClick={() => {
              handleDelete(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <Button type="primary" key="add" onClick={() => handleEditGroup({})}>
            <FormattedMessage id="plugin.group.add" />
          </Button>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetUserGroups(params);
        }}
        columnsState={{
          persistenceKey: 'user-group-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
        }}
      />
      {editVisible && (
        <UserGroupForm
          open={editVisible}
          group={currentGroup}
          onCancel={() => {
            setEditVisible(false);
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
    </PageContainer>
  );
};

export default PluginUserGroup;
