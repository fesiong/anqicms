import {
  deleteSettingNavType,
  getSettingNavTypes,
  saveSettingNavType,
} from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Input, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

export type navTypesProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const NavTypes: React.FC<navTypesProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisbile, setEditVisible] = useState<boolean>(false);
  const [editingType, setEditingType] = useState<any>({});
  const [editingInput, setEditingInput] = useState<string>('');
  const intl = useIntl();

  const handleAddType = () => {
    setEditingType({});
    setEditingInput('');
    setEditVisible(true);
  };

  const handleEditType = (record: any) => {
    setEditingType(record);
    setEditingInput(record.title);
    setEditVisible(true);
  };

  const handleRemove = async (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        let res = await deleteSettingNavType(record);

        message.info(res.msg);
        actionRef.current?.reloadAndRest?.();
      },
    });
  };

  const handleSaveType = () => {
    saveSettingNavType({
      id: editingType.id,
      title: editingInput,
    })
      .then((res) => {
        if (res.code === 0) {
          setEditVisible(false);

          actionRef.current?.reloadAndRest?.();
        } else {
          message.error(res.msg);
        }
      })
      .catch((err) => {
        console.log(err);
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
    },
    {
      title: intl.formatMessage({ id: 'setting.nav.types.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 120,
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditType(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemove(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <>
      <div
        style={{ display: 'inline-block' }}
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        open={visible}
        title={
          <Button
            type="primary"
            onClick={() => {
              handleAddType();
            }}
          >
            <FormattedMessage id="setting.nav.types.add" />
          </Button>
        }
        width={600}
        onCancel={() => {
          setVisible(false);
          props.onCancel();
        }}
        footer={false}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <ProTable<any>
            actionRef={actionRef}
            rowKey="id"
            search={false}
            pagination={false}
            toolBarRender={false}
            request={(params) => {
              return getSettingNavTypes(params);
            }}
            columnsState={{
              persistenceKey: 'nav-type-table',
              persistenceType: 'localStorage',
            }}
            columns={columns}
          />
        </div>
      </Modal>
      <Modal
        open={editVisbile}
        title={
          editingType.id
            ? intl.formatMessage({
                id: 'setting.nav.types.edit',
              }) + editingType.title
            : intl.formatMessage({
                id: 'setting.nav.types.add',
              })
        }
        width={480}
        zIndex={2000}
        maskClosable={false}
        onOk={handleSaveType}
        onCancel={() => {
          setEditVisible(false);
        }}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <p>
            <FormattedMessage id="setting.nav.types.name.require" />:{' '}
          </p>
          <Input
            size="large"
            value={editingInput}
            onChange={(e) => {
              setEditingInput(e.target.value);
            }}
          />
        </div>
      </Modal>
    </>
  );
};

export default NavTypes;
