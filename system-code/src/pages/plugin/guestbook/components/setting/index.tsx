import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Input, message, Modal, Row, Space } from 'antd';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import ProForm, {
  ModalForm,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';
import { pluginGetGuestbookSetting, pluginSaveGuestbookSetting } from '@/services/plugin/guestbook';

const GuestbookSetting: React.FC = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [setting, setSetting] = useState<any>({ fields: [] });
  const [fetched, setFetched] = useState<boolean>(false);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetGuestbookSetting();
    let setting = res.data || { fields: [] };
    setSetting(setting);
    setFetched(true);
  };

  const handleRemoveItem = (index: number) => {
    Modal.confirm({
      title: '确定要删除该字段吗？',
      content: '你可以在保存之前，通过刷新页面来恢复。',
      onOk: async () => {
        setting.fields.splice(index, 1);
        setting.fields = [].concat(setting.fields);
        setSetting(setting);
        if (actionRef.current) {
          actionRef.current.reload();
        }
      },
    });
  };

  const handleSaveField = async (values: any) => {
    let exists = false;
    for (let i in setting.fields) {
      if (setting.fields[i].field_name == values.field_name) {
        exists = true;
        setting.fields[i] = values;
      }
    }
    if (!exists) {
      setting.fields.push(values);
    }
    setting.fields = [].concat(setting.fields);
    setSetting(setting);
    if (actionRef.current) {
      actionRef.current.reload();
    }
    setEditVisible(false);
  };

  const handleChangeReturnMessage = (e: any) => {
    setting.return_message = e.target.value;
    setSetting(setting);
  };

  const handleSaveSetting = async () => {
    let res = await pluginSaveGuestbookSetting(setting);

    if (res.code === 0) {
      message.success(res.msg);
      setEditVisible(false);
      if (actionRef.current) {
        actionRef.current.reload();
      }
    } else {
      message.error(res.msg);
    }
  };

  const columns: ProColumns<any>[] = [
    {
      title: '参数名称',
      dataIndex: 'name',
    },
    {
      title: '调用字段',
      dataIndex: 'field_name',
    },
    {
      title: '字段类型',
      dataIndex: 'type',
      render: (text: any, record) => (
        <div>
          <span>{record.is_system ? '(内置)' : ''}</span>
          <span>{text}</span>
        </div>
      ),
    },
    {
      title: '是否必填',
      dataIndex: 'required',

      valueEnum: {
        false: {
          text: '选填',
          status: 'Default',
        },
        true: {
          text: '必填',
          status: 'Success',
        },
      },
    },
    {
      title: '操作',
      dataIndex: 'option',
      render: (text: any, record, index) => (
        <Space size={20}>
          {!record.is_system && (
            <>
              <a
                onClick={() => {
                  setCurrentField(record);
                  setEditVisible(true);
                }}
              >
                编辑
              </a>
              <a
                className="text-red"
                onClick={() => {
                  handleRemoveItem(index);
                }}
              >
                删除
              </a>
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        width={800}
        title="网站留言设置"
        visible={visible}
        onCancel={() => {
          setVisible(false);
        }}
        okText="保存"
        onOk={() => {
          handleSaveSetting();
        }}
      >
        {fetched && (
          <Row gutter={16}>
            <Col>
              <div style={{ lineHeight: '32px' }}>留言成功提示:</div>
            </Col>
            <Col flex={1}>
              <Input
                name="return_message"
                defaultValue={setting.return_message}
                placeholder={'默认：感谢您的留言！'}
                onChange={handleChangeReturnMessage}
              />
              <div className="text-muted">用户提交留言后看到的提示。例如：感谢您的留言！</div>
            </Col>
          </Row>
        )}
        <ProTable<any>
          rowKey="name"
          search={false}
          actionRef={actionRef}
          toolBarRender={() => [
            <Button
              key="add"
              type="primary"
              onClick={() => {
                setCurrentField({});
                setEditVisible(true);
              }}
            >
              新增字段
            </Button>,
          ]}
          tableAlertRender={false}
          tableAlertOptionRender={false}
          request={async (params, sort) => {
            return {
              data: setting.fields || [],
              success: true,
            };
          }}
          columns={columns}
          pagination={false}
        />
      </Modal>
      {editVisible && (
        <ModalForm
          width={600}
          title={currentField.name ? currentField.name + '修改字段' : '添加字段'}
          visible={editVisible}
          modalProps={{
            onCancel: () => {
              setEditVisible(false);
            },
          }}
          initialValues={currentField}
          layout="horizontal"
          onFinish={async (values) => {
            handleSaveField(values);
          }}
        >
          <ProFormText name="name" required label="参数名" extra="如：文章作者、类型、内容来源等" />
          <ProFormText
            name="field_name"
            label="调用字段"
            disabled={currentField.field_name ? true : false}
            extra="英文字母开头，只能填写字母和数字，默认为参数名称的拼音"
          />
          <ProFormRadio.Group
            name="type"
            label="字段类型"
            disabled={currentField.field_name ? true : false}
            valueEnum={{
              text: '单行文本',
              number: '数字',
              textarea: '多行文本',
              radio: '单项选择',
              checkbox: '多项选择',
              select: '下拉选择',
            }}
          />
          <ProFormRadio.Group
            name="required"
            label="是否必填"
            options={[
              { label: '选填', value: false },
              { label: '必填', value: true },
            ]}
          />
          <ProFormTextArea
            label="默认值"
            name="content"
            fieldProps={{
              rows: 4,
            }}
            extra="单选、多选、下拉的多个值，一行一个。"
          />
        </ModalForm>
      )}
    </>
  );
};

export default GuestbookSetting;
