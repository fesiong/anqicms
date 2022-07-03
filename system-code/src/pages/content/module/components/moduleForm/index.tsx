import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Input, message, Modal, Row, Space, Tag } from 'antd';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import {
  ModalForm,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';
import { deleteModuleField, getModuleInfo, saveModule } from '@/services';

export type ModuleFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  type: number;
  visible: boolean;
  module: any;
};

var submitting = false

const ModuleForm: React.FC<ModuleFormProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [setting, setSetting] = useState<any>({ fields: [] });
  const [fetched, setFetched] = useState<boolean>(false);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    if (props.module.id) {
      const res = await getModuleInfo({id: props.module.id});
      let setting = res.data || { fields: [] };
      setSetting(setting);
    }
    setFetched(true);
  };

  const handleRemoveItem = (record: any, index: number) => {
    Modal.confirm({
      title: '确定要删除该字段吗？',
      content: '对应的文档内该字段内容也会被删除',
      onOk: async () => {
        deleteModuleField({id: props.module.id, field_name: record.field_name})
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
    let reg = /^[a-z][0-9a-z_]+$/
    if(!values.field_name || !reg.test(values.field_name)) {
      message.error('调用字段必须是英文字母')
      return
    }
    let exists = false;
    if (!setting.fields) {
      setting.fields = [];
    }
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

  const handleChangeInput = (field: string, e: any) => {
    setting[field] = e.target.value;
    setSetting(setting);
  };

  const handleSaveSetting = async () => {
    if (submitting) {
      return;
    }
    submitting = true;
    let hide = message.loading('正在提交中', 0)
    saveModule(setting).then(res => {
      if (res.code === 0) {
        message.success(res.msg);
        setEditVisible(false);
        if (actionRef.current) {
          actionRef.current.reload();
        }
        props.onSubmit();
      } else {
        message.error(res.msg);
      }
    }).finally(() => {
      submitting = false;
      hide();
    })
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
                  handleRemoveItem(record, index);
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
      <Modal
        width={800}
        title="模型设置"
        visible={props.visible}
        onCancel={() => {
          props.onCancel();
        }}
        cancelText='关闭'
        okText="保存"
        onOk={() => {
          handleSaveSetting();
        }}
      >
        {fetched && (
          <div>
          <div>
            <Row gutter={16}>
              <Col>
                <div style={{ lineHeight: '32px', width: '120px' }}>模型名称:</div>
              </Col>
              <Col flex={1}>
                <Input
                  name="title"
                  defaultValue={setting.title}
                  onChange={(e: any) => {handleChangeInput('title', e)}}
                />
              </Col>
            </Row>
            <Row className='mt-normal' gutter={16}>
            <Col>
              <div style={{ lineHeight: '32px', width: '120px' }}>模型表名:</div>
            </Col>
            <Col flex={1}>
              <Input
                name="table_name"
                defaultValue={setting.table_name}
                onChange={(e: any) => {handleChangeInput('table_name', e)}}
              />
              <div className="text-muted">仅支持英文小写字母。</div>
            </Col>
          </Row>
            <Row className='mt-normal' gutter={16}>
            <Col>
              <div style={{ lineHeight: '32px', width: '120px' }}>URL别名:</div>
            </Col>
            <Col flex={1}>
              <Input
                name="url_token"
                defaultValue={setting.url_token}
                onChange={(e: any) => {handleChangeInput('url_token', e)}}
              />
              <div className="text-muted">仅支持英文小写字母，伪静态规则定义的 <Tag>{'{module}'}</Tag>调用。</div>
            </Col>
          </Row>
          <Row className='mt-normal' gutter={16}>
            <Col>
              <div style={{ lineHeight: '32px', width: '120px' }}>标题名称:</div>
            </Col>
            <Col flex={1}>
              <Input
                name="title_name"
                defaultValue={setting.title_name}
                onChange={(e: any) => {handleChangeInput('title_name', e)}}
              />
              <div className="text-muted">显示在发布文档的时候的标题提示位置。</div>
            </Col>
          </Row>
          </div>
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
        </div>
        )}
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
              image: '图片',
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

export default ModuleForm;
