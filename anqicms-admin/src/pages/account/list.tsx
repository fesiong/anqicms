import {
  deleteAdminInfo,
  getAdminGroups,
  getAdminList,
  saveAdmin,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

const AdminList: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const actionRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [editInfo, setEditInfo] = useState<any>({});
  const intl = useIntl();

  const { currentUser } = initialState;

  const handleEdit = (record: any) => {
    setEditInfo(record);
    setEditVisible(true);
  };

  const onSubmitEdit = async (values: any) => {
    if (currentUser.id === editInfo.id || editInfo.id === 1) {
      // 自己无法禁用自己
      values.status = 1;
    }
    values.status = Number(values.status);
    const postData = Object.assign(editInfo, values);
    saveAdmin(postData).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
      } else {
        message.info(res.msg);
        actionRef.current?.reload();
        setEditVisible(false);
      }
    });
  };

  const handleRemove = (record: any) => {
    if (currentUser.id === record.id || record.id === 1) {
      message.error(intl.formatMessage({ id: 'account.list.cannot-delete' }));
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'account.list.confirm-delete' }),
      onOk: () => {
        deleteAdminInfo({
          id: record.id,
        }).then((res) => {
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
      title: intl.formatMessage({ id: 'account.base.username' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'account.base.group' }),
      dataIndex: 'group.title',
      render: (text, record) => record.group?.title || text,
    },
    {
      title: intl.formatMessage({ id: 'account.list.create-time' }),
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'account.list.login-time' }),
      dataIndex: 'login_time',
      render: (text, record) =>
        dayjs(record.login_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'setting.content.notenable' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'setting.content.enable' }),
          status: 'Success',
        },
      },
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
              handleEdit(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.id]);
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
        headerTitle={intl.formatMessage({ id: 'account.list.name' })}
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params) => {
          return getAdminList(params);
        }}
        columnsState={{
          persistenceKey: 'account-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEdit({});
            }}
          >
            <PlusOutlined /> <FormattedMessage id="account.list.add" />
          </Button>,
        ]}
      />
      {editVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'account.list.edit' })}
          open={editVisible}
          initialValues={editInfo}
          onFinish={onSubmitEdit}
          onOpenChange={(e) => setEditVisible(e)}
        >
          <ProFormText
            name="user_name"
            label={intl.formatMessage({ id: 'account.base.username' })}
            width="lg"
          />
          <ProFormText.Password
            name="password"
            label={intl.formatMessage({ id: 'account.list.password' })}
            width="lg"
            placeholder={intl.formatMessage({
              id: 'account.list.password.placeholder',
            })}
          />
          <ProFormSelect
            name="group_id"
            label={intl.formatMessage({ id: 'account.base.group' })}
            request={async () => {
              const res = await getAdminGroups({});
              return res.data || [];
            }}
            fieldProps={{
              fieldNames: {
                label: 'title',
                value: 'id',
              },
            }}
          />
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'website.status' })}
            name="status"
            valueEnum={{
              0: intl.formatMessage({ id: 'setting.content.notenable' }),
              1: intl.formatMessage({ id: 'setting.content.enable' }),
            }}
          />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default AdminList;
