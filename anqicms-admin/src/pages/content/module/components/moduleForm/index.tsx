import { deleteModuleField, getModuleInfo, saveModule } from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Button,
  Col,
  Input,
  Modal,
  Radio,
  Row,
  Space,
  Tag,
  message,
} from 'antd';
import React, { useEffect, useRef, useState } from 'react';

export type ModuleFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  type: number;
  open: boolean;
  module: any;
};

let submitting = false;

const ModuleForm: React.FC<ModuleFormProps> = (props) => {
  const actionArchiveRef = useRef<ActionType>();
  const actionCategoryRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [setting, setSetting] = useState<any>({ fields: [] });
  const [fetched, setFetched] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    if (props.module.id) {
      const res = await getModuleInfo({ id: props.module.id });
      let setting = res.data || { fields: [] };
      // 迁移前的模型没有 kind。归一成 article 再进表单：空串提交上去会被后端
      // 当成非法类型报错，而用户只是打开看了一眼、什么都没改。
      if (!setting.kind) {
        setting.kind = 'article';
      }
      setSetting(setting);
    } else {
      setSetting({ status: 1, kind: 'article' });
    }
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleRemoveItem = (record: any, index: number) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.module.field.delete.confirm' }),
      content: intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: async () => {
        if (record.form === 'archive') {
          // archive 需要请求后端
          deleteModuleField({
            id: props.module.id,
            field_name: record.field_name,
          });
          setting.fields.splice(index, 1);
          setting.fields = [].concat(setting.fields);
        } else if (record.form === 'category') {
          // category不需要请求后端
          setting.category_fields.splice(index, 1);
          setting.category_fields = [].concat(setting.category_fields);
        }
        setSetting(setting);
        if (actionArchiveRef.current) {
          actionArchiveRef.current.reload();
        }
        if (actionCategoryRef.current) {
          actionCategoryRef.current.reload();
        }
      },
    });
  };

  const handleSaveField = async (values: any) => {
    let reg = /^[a-z][0-9a-z_]+$/;
    if (!values.field_name || !reg.test(values.field_name)) {
      message.error(intl.formatMessage({ id: 'content.module.field.error' }));
      return;
    }
    let exists = false;
    if (currentField.form === 'archive') {
      if (!setting.fields) {
        setting.fields = [];
      }
      for (let i in setting.fields) {
        if (setting.fields[i].field_name === values.field_name) {
          exists = true;
          setting.fields[i] = values;
        }
      }
      if (!exists) {
        setting.fields.push(values);
      }
      setting.fields = [].concat(setting.fields);
    } else if (currentField.form === 'category') {
      if (!setting.category_fields) {
        setting.category_fields = [];
      }
      for (let i in setting.category_fields) {
        if (setting.category_fields[i].field_name === values.field_name) {
          exists = true;
          setting.category_fields[i] = values;
        }
      }
      if (!exists) {
        setting.category_fields.push(values);
      }
      setting.category_fields = [].concat(setting.category_fields);
    }
    setSetting(setting);
    if (actionArchiveRef.current) {
      actionArchiveRef.current.reload();
    }
    if (actionCategoryRef.current) {
      actionCategoryRef.current.reload();
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
    let hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveModule(setting)
      .then((res) => {
        if (res.code === 0) {
          message.success(res.msg);
          setEditVisible(false);
          if (actionArchiveRef.current) {
            actionArchiveRef.current.reload();
          }
          if (actionCategoryRef.current) {
            actionCategoryRef.current.reload();
          }
          props.onSubmit();
        } else {
          message.error(res.msg);
        }
      })
      .finally(() => {
        submitting = false;
        hide();
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'content.module.field.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'content.module.field.field-name' }),
      dataIndex: 'field_name',
    },
    {
      title: intl.formatMessage({ id: 'content.module.field.type' }),
      dataIndex: 'type',
      render: (text: any, record) => (
        <div>
          <span>
            {record.is_system
              ? intl.formatMessage({ id: 'content.module.field.type.built-in' })
              : ''}
          </span>
          <span>{text}</span>
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'content.module.field.isrequired' }),
      dataIndex: 'required',

      valueEnum: {
        false: {
          text: intl.formatMessage({
            id: 'content.module.field.isrequired.no',
          }),
          status: 'Default',
        },
        true: {
          text: intl.formatMessage({
            id: 'content.module.field.isrequired.yes',
          }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
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
                <FormattedMessage id="setting.action.edit" />
              </a>
              <a
                className="text-red"
                onClick={() => {
                  handleRemoveItem(record, index);
                }}
              >
                <FormattedMessage id="setting.system.delete" />
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
        width={1000}
        title={intl.formatMessage({ id: 'content.module.setting' })}
        open={props.open}
        onCancel={() => {
          props.onCancel();
        }}
        onOk={() => {
          handleSaveSetting();
        }}
      >
        {fetched && (
          <div>
            <div>
              <Row gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.module.title" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Input
                    name="name"
                    defaultValue={setting.name}
                    onChange={(e: any) => {
                      handleChangeInput('name', e);
                    }}
                  />
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.seo-title.name" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Input
                    name="title"
                    defaultValue={setting.title}
                    onChange={(e: any) => {
                      handleChangeInput('title', e);
                    }}
                  />
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.module.kind" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Radio.Group
                    name="kind"
                    defaultValue={setting.kind}
                    onChange={(e: any) => {
                      handleChangeInput('kind', e);
                    }}
                    options={[
                      {
                        value: 'article',
                        label: intl.formatMessage({
                          id: 'content.module.kind.article',
                        }),
                      },
                      {
                        value: 'product',
                        label: intl.formatMessage({
                          id: 'content.module.kind.product',
                        }),
                      },
                    ]}
                  />
                  <div className="text-muted">
                    <FormattedMessage id="content.module.kind.description" />
                  </div>
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.module.field" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Input
                    name="table_name"
                    defaultValue={setting.table_name}
                    onChange={(e: any) => {
                      handleChangeInput('table_name', e);
                    }}
                  />
                  <div className="text-muted">
                    <FormattedMessage id="content.module.field.description" />
                  </div>
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.url-token.name" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Input
                    name="url_token"
                    defaultValue={setting.url_token}
                    onChange={(e: any) => {
                      handleChangeInput('url_token', e);
                    }}
                  />
                  <div className="text-muted">
                    <FormattedMessage id="content.module.url-token.tips.before" />{' '}
                    <Tag>{'{module}'}</Tag>
                    <FormattedMessage id="content.module.url-token.tips.after" />
                  </div>
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.category.status" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Radio.Group
                    name="status"
                    defaultValue={setting.status}
                    onChange={(e: any) => {
                      handleChangeInput('status', e);
                    }}
                    options={[
                      {
                        value: 0,
                        label: intl.formatMessage({
                          id: 'content.category.status.hide',
                        }),
                      },
                      {
                        value: 1,
                        label: intl.formatMessage({
                          id: 'content.category.status.ok',
                        }),
                      },
                    ]}
                  />
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.keywords.name" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Input
                    name="keywords"
                    defaultValue={setting.keywords}
                    onChange={(e: any) => {
                      handleChangeInput('keywords', e);
                    }}
                  />
                </Col>
              </Row>
              <Row className="mt-normal" gutter={16}>
                <Col>
                  <div style={{ lineHeight: '32px', width: '120px' }}>
                    <FormattedMessage id="content.description.name" />:
                  </div>
                </Col>
                <Col flex={1}>
                  <Input.TextArea
                    name="description"
                    defaultValue={setting.description}
                    onChange={(e: any) => {
                      handleChangeInput('description', e);
                    }}
                  />
                </Col>
              </Row>
            </div>
            <ProTable<any>
              headerTitle={intl.formatMessage({
                id: 'content.module.fields.name',
              })}
              key="fields-table"
              rowKey="name"
              search={false}
              actionRef={actionArchiveRef}
              toolBarRender={() => [
                <Button
                  key="add"
                  type="primary"
                  onClick={() => {
                    setCurrentField({
                      form: 'archive',
                      type: 'text',
                      required: false,
                      follow_level: false,
                      is_filter: false,
                    });
                    setEditVisible(true);
                  }}
                >
                  <FormattedMessage id="content.module.field.add" />
                </Button>,
              ]}
              tableAlertRender={false}
              tableAlertOptionRender={false}
              request={async () => {
                return {
                  data: (setting.fields || []).map(
                    (item: any) => ((item.form = 'archive'), item),
                  ),
                  success: true,
                };
              }}
              columnsState={{
                persistenceKey: 'module-fields-table',
                persistenceType: 'localStorage',
              }}
              columns={columns}
              pagination={false}
            />
            <ProTable<any>
              headerTitle={intl.formatMessage({
                id: 'content.module.category_fields.name',
              })}
              key="category_fields-table"
              rowKey="name"
              search={false}
              actionRef={actionCategoryRef}
              toolBarRender={() => [
                <Button
                  key="add"
                  type="primary"
                  onClick={() => {
                    setCurrentField({
                      form: 'category',
                      type: 'text',
                      required: false,
                      follow_level: false,
                      is_filter: false,
                    });
                    setEditVisible(true);
                  }}
                >
                  <FormattedMessage id="content.module.field.add" />
                </Button>,
              ]}
              tableAlertRender={false}
              tableAlertOptionRender={false}
              request={async () => {
                return {
                  data: (setting.category_fields || []).map(
                    (item: any) => ((item.form = 'category'), item),
                  ),
                  success: true,
                };
              }}
              columnsState={{
                persistenceKey: 'module-fields-table',
                persistenceType: 'localStorage',
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
          title={
            currentField.name
              ? currentField.name +
                intl.formatMessage({ id: 'content.module.field.edit' })
              : intl.formatMessage({ id: 'content.module.field.add' })
          }
          open={editVisible}
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
          <ProFormText
            name="name"
            required
            label={intl.formatMessage({ id: 'content.module.field.name' })}
            extra={intl.formatMessage({
              id: 'content.module.field.name.description',
            })}
          />
          <ProFormText
            name="field_name"
            label={intl.formatMessage({
              id: 'content.module.field.field-name',
            })}
            disabled={currentField.field_name ? true : false}
            extra={intl.formatMessage({
              id: 'content.module.field.field-name.description',
            })}
          />
          <ProFormRadio.Group
            name="type"
            label={intl.formatMessage({ id: 'content.module.field.type' })}
            disabled={currentField.field_name ? true : false}
            valueEnum={{
              text: intl.formatMessage({
                id: 'content.module.field.type.text',
              }),
              number: intl.formatMessage({
                id: 'content.module.field.type.number',
              }),
              textarea: intl.formatMessage({
                id: 'content.module.field.type.textarea',
              }),
              editor: intl.formatMessage({
                id: 'content.module.field.type.editor',
              }),
              radio: intl.formatMessage({
                id: 'content.module.field.type.radio',
              }),
              checkbox: intl.formatMessage({
                id: 'content.module.field.type.checkbox',
              }),
              select: intl.formatMessage({
                id: 'content.module.field.type.select',
              }),
              image: intl.formatMessage({
                id: 'content.module.field.type.image',
              }),
              images: intl.formatMessage({
                id: 'content.module.field.type.images',
              }),
              file: intl.formatMessage({
                id: 'content.module.field.type.file',
              }),
              texts: intl.formatMessage({
                id: 'content.module.field.type.texts',
              }),
              archive: intl.formatMessage({
                id: 'content.module.field.type.archive',
              }),
              category: intl.formatMessage({
                id: 'content.module.field.type.category',
              }),
              date: intl.formatMessage({
                id: 'content.module.field.type.date',
              }),
              time: intl.formatMessage({
                id: 'content.module.field.type.time',
              }),
              datetime: intl.formatMessage({
                id: 'content.module.field.type.datetime',
              }),
              color: intl.formatMessage({
                id: 'content.module.field.type.color',
              }),
              timeline: intl.formatMessage({
                id: 'content.module.field.type.timeline',
              }),
            }}
          />
          <ProFormRadio.Group
            name="required"
            label={intl.formatMessage({
              id: 'content.module.field.isrequired',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isrequired.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isrequired.yes',
                }),
                value: true,
              },
            ]}
          />
          <ProFormRadio.Group
            name="follow_level"
            label={intl.formatMessage({ id: 'content.read-level.name' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.level.none',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.level.follow',
                }),
                value: true,
              },
            ]}
          />
          <ProFormRadio.Group
            name="is_filter"
            label={intl.formatMessage({ id: 'content.module.field.isfilter' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isfilter.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isfilter.yes',
                }),
                value: true,
              },
            ]}
            extra={intl.formatMessage({
              id: 'content.module.field.isfilter.description',
            })}
          />
          <ProFormTextArea
            label={intl.formatMessage({ id: 'content.param.default' })}
            name="content"
            fieldProps={{
              rows: 4,
            }}
            extra={intl.formatMessage({
              id: 'content.module.field.default.description',
            })}
          />
        </ModalForm>
      )}
    </>
  );
};

export default ModuleForm;
