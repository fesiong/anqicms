import React, { useRef, useState } from 'react';
import { Button, Input, message, Modal, Space } from 'antd';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import { deleteSettingNavType, getSettingNavTypes, saveSettingNavType } from '@/services';

export type navTypesProps = {
  onCancel: (flag?: boolean) => void;
};

const NavTypes: React.FC<navTypesProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisbile, setEditVisible] = useState<boolean>(false);
  const [editingType, setEditingType] = useState<any>({});
  const [editingInput, setEditingInput] = useState<string>('');

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
    let res = await deleteSettingNavType(record);

    message.info(res.msg);
    actionRef.current?.reloadAndRest?.();
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
      title: '导航名称',
      dataIndex: 'title',
    },
    {
      title: '操作',
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
            编辑
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove(record);
              actionRef.current?.reloadAndRest?.();
            }}
          >
            删除
          </a>
        </Space>
      ),
    },
  ];

  return (
    <>
      <div
        style={{display: 'inline-block'}}
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        visible={visible}
        title={
          <Button
            type="primary"
            onClick={() => {
              handleAddType();
            }}
          >
            新增导航类别
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
            request={(params, sort) => {
              return getSettingNavTypes(params);
            }}
            columns={columns}
          />
        </div>
      </Modal>
      <Modal
        visible={editVisbile}
        title={editingType.id ? '重命名类别：' + editingType.title : '新增导航类别'}
        width={480}
        zIndex={2000}
        okText="确认"
        cancelText="取消"
        maskClosable={false}
        onOk={handleSaveType}
        onCancel={() => {
          setEditVisible(false);
        }}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <p>请填写名称: </p>
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
