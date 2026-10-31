import NewContainer from '@/components/NewContainer';
import AttachmentSelect from '@/components/attachment';
import {
  getArchives,
  getCategories,
  getSettingContent,
  getSettingDiyFields,
  saveSettingDiyFields,
} from '@/services';
import {
  DeleteOutlined,
  DownOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
  UpOutlined,
} from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  Divider,
  Image,
  Input,
  InputRef,
  message,
  Modal,
  Row,
  Space,
  Tag,
} from 'antd';
import React, { lazy, Suspense, useEffect, useRef, useState } from 'react';
import './index.less';
const MarkdownEditor = lazy(() => import('@/components/markdown'));
const NewAiEditor = lazy(() => import('@/components/newAiEditor'));
const SimpleEditor = lazy(() => import('@/components/simpleEditor'));

const SettingDiyFieldFrom: React.FC<any> = () => {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();

  const [currentGroup, setCurrentGroup] = useState<string>('');
  const [groupNames, setGroupNames] = useState<string[]>([]);
  const [groupName, setGroupName] = useState('');
  const inputRef = useRef<InputRef>(null);

  const [addFieldOpen, setAddFieldOpen] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [currentIndex, setCurrentIndex] = useState<number>(-1);
  const [contentSetting, setContentSetting] = useState<any>({});
  const [categories, setCategories] = useState<any>([]);
  const [setting, setSetting] = useState<any[]>([]);
  const [fieldsOpen, setFieldsOpen] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);
  const [selectedArchives, setSelectedArchives] = useState<any[]>([]);

  const getSelectedArchives = (arcIds: number[]) => {
    if (arcIds.length > 0) {
      // 存在了再处理
      getArchives({
        id: arcIds.join(','),
        limit: 20,
      }).then((res) => {
        if (res.data) {
          setSelectedArchives(res.data);
          setSearchArchives(res.data);
        }
      });
    }
  };

  const getSetting = async () => {
    getSettingContent().then(async (res) => {
      setContentSetting(res.data || {});
      const res2 = await getSettingDiyFields();
      const data = res2.data || [];
      const arcIds = [];
      const newSet = new Set<string>();
      let hasEmpty = false;
      for (const item of data) {
        if (item.group) {
          newSet.add(item.group);
        } else {
          hasEmpty = true;
        }
        if (item.type === 'archive' && item.value?.length > 0) {
          arcIds.push(...item.value);
        }
      }
      getSelectedArchives(arcIds);
      getCategories().then((res) => {
        setCategories(res.data || []);
      });
      if (hasEmpty || newSet.size === 0) {
        newSet.add('');
      }
      setGroupNames([...newSet]);
      setCurrentGroup(newSet.values().next().value as string);
      setSetting(data);
    });
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleEditField = (index: number) => {
    if (index === -1) {
      setCurrentField({ type: 'text', required: false });
    } else {
      setCurrentField(setting[index]);
    }
    setCurrentIndex(index);
    setAddFieldOpen(true);
  };

  const capitalizeFirstLetter = (str: string) => {
    if (!str) return '';
    return str.charAt(0).toUpperCase() + str.slice(1);
  };

  const handleSaveField = (values: any) => {
    let name = (values.name || '').trim();
    if (name.length === 0) {
      return false;
    }
    const normalizedValues = {
      ...values,
      name: capitalizeFirstLetter(name),
      type: values.type || 'text',
    };

    setSetting((prevSetting) => {
      // ✅ 检查重复名称
      const isDuplicate = prevSetting.some(
        (item: any) => item.name === normalizedValues.name,
      );

      if (currentIndex === -1 && isDuplicate) {
        message.error(
          intl.formatMessage({ id: 'setting.diyfield.name-duplicate' }),
        );
        return prevSetting;
      }

      // ✅ 不可变方式更新数组
      if (currentIndex === -1) {
        return [...prevSetting, normalizedValues];
      } else {
        return prevSetting.map((item: any, index: number) =>
          index === currentIndex ? { ...item, ...normalizedValues } : item,
        );
      }
    });
    actionRef.current?.reload();
    setAddFieldOpen(false);
  };

  const handleCleanExtraField = (index: number) => {
    setting[index].value = '';
    setSetting([].concat(...setting));
  };

  const handleUpdateFieldValue = (index: number, val: any) => {
    const values = setting[index];
    setting[index].value = val;
    if (values.type !== 'editor') {
      setSetting([].concat(...setting));
    }
  };

  const handleMoveExtraFieldItem = (
    index: number,
    idx: number,
    direction: 'up' | 'down',
  ) => {
    if (direction === 'up') {
      if (idx <= 0) {
        return;
      }
      const temp = setting[index].value[idx];
      setting[index].value[idx] = setting[index].value[idx - 1];
      setting[index].value[idx - 1] = temp;
    } else {
      if (idx >= setting[index].value.length - 1) {
        return;
      }
      const temp = setting[index].value[idx];
      setting[index].value[idx] = setting[index].value[idx + 1];
      setting[index].value[idx + 1] = temp;
    }
    setSetting([].concat(...setting));
  };

  const handleCleanExtraFieldItem = (index: number, idx: number) => {
    setting[index].value.splice(idx, 1);
    setSetting([].concat(...setting));
  };

  const handleUploadExtraFieldItem = (index: number, rows: any) => {
    if (!setting[index].value) {
      setting[index].value = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in setting[index].value) {
        if (setting[index].value[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        setting[index].value.push(row.file_path);
      }
    }

    setSetting([].concat(...setting));
  };

  const updateExtraContent = async (index: number, html: string) => {
    handleUpdateFieldValue(index, html);
  };

  const handleUploadExtraField = (index: number, row: any) => {
    handleUpdateFieldValue(index, row.file_path);
  };

  const handleDeleteField = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'setting.system.confirm-delete-param',
      }),
      onOk: () => {
        setting.splice(index, 1);
        setSetting([].concat(...setting));
      },
    });
  };

  const onAddExtraTextsField = (index: number) => {
    if (!setting[index].value) {
      setting[index].value = [];
    }
    setting[index].value.push({
      key: '',
      value: '',
    });
    formRef?.current?.setFieldsValue(setting);
    setSetting([].concat(...setting));
  };

  const onChangeExtraTextsField = (
    index: number,
    idx: number,
    keyName: any,
    value: any,
  ) => {
    setting[index].value[idx][keyName] = value;
    setSetting([].concat(...setting));
  };

  const onMoveUpExtraTextsField = (index: number, idx: number) => {
    // 移动
    if (idx > 0) {
      const tmp = setting[index].value[idx];
      setting[index].value[idx] = setting[index].value[idx - 1];
      setting[index].value[idx - 1] = tmp;
      formRef?.current?.setFieldsValue(setting);
      setSetting([].concat(...setting));
    }
  };

  const onMoveDownExtraTextsField = (index: number, idx: number) => {
    // 移动
    if (idx < setting[index].value.length - 1) {
      const tmp = setting[index].value[idx];
      setting[index].value[idx] = setting[index].value[idx + 1];
      setting[index].value[idx + 1] = tmp;
      formRef?.current?.setFieldsValue(setting);
      setSetting([].concat(...setting));
    }
  };

  const onRemoveExtraTextsField = (index: number, idx: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.module.field.delete.confirm',
      }),
      content: intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: () => {
        if (setting[index].value.length === 1) {
          setting[index].value = [];
        } else {
          setting[index].value.splice(idx, 1);
        }
        formRef?.current?.setFieldsValue(setting);
        setSetting([].concat(...setting));
      },
    });
  };

  const onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      if (selectedArchives) {
        searchItems.push(...selectedArchives);
      } else {
        searchItems.push({
          id: 0,
          title: intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        });
      }
      setSearchArchives(searchItems.concat(res.data || []));
    });
  };

  const onSubmit = () => {
    // 提交
    saveSettingDiyFields(setting).then((res) => {
      message.success(res.msg);
    });
  };

  const onGroupNameChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setGroupName(event.target.value);
  };

  const addGroup = (
    e: React.MouseEvent<HTMLButtonElement | HTMLAnchorElement>,
  ) => {
    e.preventDefault();
    if (!groupName) {
      message.error(intl.formatMessage({ id: 'setting.banner.name.require' }));
      return;
    }
    // default
    const newSet = new Set<string>(groupNames);
    newSet.add(groupName);
    setGroupNames([...newSet]);
    setGroupName('');
    setTimeout(() => {
      inputRef.current?.focus();
    }, 0);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'setting.system.remark' }),
      dataIndex: 'remark',
    },
    {
      title: intl.formatMessage({ id: 'setting.system.param-name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'content.module.field.group' }),
      dataIndex: 'group',
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
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      render: (text: any, record, index) => (
        <Space size={20}>
          {!record.is_system && (
            <>
              <a
                onClick={() => {
                  handleEditField(index);
                }}
              >
                <FormattedMessage id="setting.action.edit" />
              </a>
              <a
                className="text-red"
                onClick={() => {
                  handleDeleteField(index);
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
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        title={
          <div className="module-tags">
            {groupNames.map((item: any, index: number) => (
              <div
                className={
                  'module-tag ' + (item === currentGroup ? 'active' : '')
                }
                key={index + item}
                onClick={() => {
                  setCurrentGroup(item);
                }}
              >
                {item || intl.formatMessage({ id: 'design.diyfield.ungrouped' })}
              </div>
            ))}
          </div>
        }
        extra={
          <Button
            onClick={() => {
              setFieldsOpen(true);
            }}
          >
            <FormattedMessage id="setting.system.diy-params" />
          </Button>
        }
      >
        <ProForm layout="vertical" formRef={formRef} onFinish={onSubmit}>
          <Row>
            {setting.map(
              (item: any, index: number) =>
                item.group === currentGroup && (
                  <Col sm={24} xs={24} key={index}>
                    {item.type === 'text' ? (
                      <ProFormText
                        name={[index, 'value']}
                        width="lg"
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        fieldProps={{
                          value: item.value,
                          onChange: (e: any) => {
                            handleUpdateFieldValue(index, e.target.value);
                          },
                        }}
                        placeholder={
                          item.content &&
                          intl.formatMessage({
                            id: 'content.param.default',
                          }) + item.content
                        }
                      />
                    ) : item.type === 'number' ? (
                      <ProFormDigit
                        name={[index, 'value']}
                        width="lg"
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        fieldProps={{
                          value: item.value,
                          onChange: (e: any) => {
                            handleUpdateFieldValue(index, e);
                          },
                        }}
                        placeholder={
                          item.content &&
                          intl.formatMessage({
                            id: 'content.param.default',
                          }) + item.content
                        }
                      />
                    ) : item.type === 'textarea' ? (
                      <ProFormTextArea
                        name={[index, 'value']}
                        width="lg"
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        fieldProps={{
                          value: item.value,
                          onChange: (e: any) => {
                            handleUpdateFieldValue(index, e.target.value);
                          },
                        }}
                        placeholder={
                          item.content &&
                          intl.formatMessage({
                            id: 'content.param.default',
                          }) + item.content
                        }
                      />
                    ) : item.type === 'editor' ? (
                      <ProFormText
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        extra={
                          item.content &&
                          intl.formatMessage({
                            id: 'content.param.default',
                          }) + item.content
                        }
                      >
                        <Suspense fallback={<div style={{ height: 500 }} />}>
                          {contentSetting.editor === 'markdown' ? (
                            <MarkdownEditor
                              className="mb-normal"
                              setContent={async (val) =>
                                updateExtraContent(index, val)
                              }
                              content={item.value || ''}
                              ref={null}
                            />
                          ) : contentSetting.editor === 'simple' ? (
                            <SimpleEditor
                              className="mb-normal"
                              setContent={async (val) =>
                                updateExtraContent(index, val)
                              }
                              content={item.value || ''}
                              ref={null}
                            />
                          ) : (
                            <NewAiEditor
                              className="mb-normal"
                              setContent={async (val) => {
                                updateExtraContent(index, val);
                              }}
                              content={item.value || ''}
                              key={item.name}
                              field={item.name}
                              ref={null}
                            />
                          )}
                        </Suspense>
                      </ProFormText>
                    ) : item.type === 'radio' ? (
                      <ProFormRadio.Group
                        name={[index, 'value']}
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        fieldProps={{
                          value: item.value,
                          onChange: (e: any) => {
                            handleUpdateFieldValue(index, e.target.value);
                          },
                        }}
                        request={async () => {
                          const tmpData = item.content.split('\n');
                          const data = [];
                          for (const item1 of tmpData) {
                            data.push({ label: item1, value: item1 });
                          }
                          return data;
                        }}
                      />
                    ) : item.type === 'checkbox' ? (
                      <ProFormCheckbox.Group
                        name={[index, 'value']}
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        fieldProps={{
                          value: item.value,
                          onChange: (e: any) => {
                            handleUpdateFieldValue(index, e);
                          },
                        }}
                        request={async () => {
                          const tmpData = item.content.split('\n');
                          const data = [];
                          for (const item1 of tmpData) {
                            data.push({ label: item1, value: item1 });
                          }
                          return data;
                        }}
                      />
                    ) : item.type === 'select' ? (
                      <ProFormSelect
                        name={[index, 'value']}
                        width="lg"
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                        fieldProps={{
                          value: item.value,
                          onChange: (e: any) => {
                            handleUpdateFieldValue(index, e);
                          },
                        }}
                        request={async () => {
                          const tmpData = item.content.split('\n');
                          const data = [];
                          for (const item1 of tmpData) {
                            data.push({ label: item1, value: item1 });
                          }
                          return data;
                        }}
                      />
                    ) : item.type === 'image' ? (
                      <ProFormText
                        name={[index, 'value']}
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                      >
                        {item.value ? (
                          <div className="ant-upload-item">
                            <Image
                              preview={{
                                src: item.value,
                              }}
                              src={item.value}
                            />
                            <span
                              className="delete"
                              onClick={() => handleCleanExtraField(index)}
                            >
                              <DeleteOutlined />
                            </span>
                          </div>
                        ) : (
                          <AttachmentSelect
                            onSelect={(row) =>
                              handleUploadExtraField(index, row)
                            }
                            open={false}
                          >
                            <div className="ant-upload-item">
                              <div className="add">
                                <PlusOutlined />
                                <div style={{ marginTop: 8 }}>
                                  <FormattedMessage id="setting.system.upload" />
                                </div>
                              </div>
                            </div>
                          </AttachmentSelect>
                        )}
                      </ProFormText>
                    ) : item.type === 'images' ? (
                      <ProFormText
                        name={[index, 'value']}
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                      >
                        {item.value?.length
                          ? item.value.map((inner: string, idx: number) => (
                              <div className="ant-upload-item" key={idx}>
                                <Image
                                  preview={{
                                    src: inner,
                                  }}
                                  src={inner}
                                />
                                <div className="ant-upload-item-action">
                                  <Tag
                                    onClick={() =>
                                      handleMoveExtraFieldItem(index, idx, 'up')
                                    }
                                  >
                                    <LeftOutlined />
                                  </Tag>
                                  <Tag
                                    color="red"
                                    onClick={() =>
                                      handleCleanExtraFieldItem(index, idx)
                                    }
                                  >
                                    <DeleteOutlined />
                                  </Tag>
                                  <Tag
                                    onClick={() =>
                                      handleMoveExtraFieldItem(
                                        index,
                                        idx,
                                        'down',
                                      )
                                    }
                                  >
                                    <RightOutlined />
                                  </Tag>
                                </div>
                              </div>
                            ))
                          : null}
                        <AttachmentSelect
                          onSelect={(rows) =>
                            handleUploadExtraFieldItem(index, rows)
                          }
                          open={false}
                          multiple={true}
                        >
                          <div className="ant-upload-item">
                            <div className="add">
                              <PlusOutlined />
                              <div style={{ marginTop: 8 }}>
                                <FormattedMessage id="setting.system.upload" />
                              </div>
                            </div>
                          </div>
                        </AttachmentSelect>
                      </ProFormText>
                    ) : item.type === 'file' ? (
                      <ProFormText
                        name={[index, 'value']}
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                      >
                        {item.value ? (
                          <div className="ant-upload-item ant-upload-file">
                            <div className="field-value">{item.value}</div>
                            <span
                              className="delete"
                              onClick={() => handleCleanExtraField(index)}
                            >
                              <DeleteOutlined />
                            </span>
                          </div>
                        ) : (
                          <AttachmentSelect
                            onSelect={(row) =>
                              handleUploadExtraField(index, row)
                            }
                            open={false}
                          >
                            <Button>
                              <FormattedMessage id="setting.system.upload" />
                            </Button>
                          </AttachmentSelect>
                        )}
                      </ProFormText>
                    ) : item.type === 'texts' ? (
                      <ProFormText
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                      >
                        <div className="text-groups">
                          <div className="text-group">
                            <div className="text-key">Key</div>
                            <div className="text-value">Value</div>
                            <div className="text-action"></div>
                          </div>
                          {item.value?.length
                            ? item.value.map((inner: any, idx: number) => (
                                <div className="text-group" key={idx}>
                                  <div className="text-key">
                                    <ProFormText
                                      name={[index, 'value', idx, 'key']}
                                      fieldProps={{
                                        value: inner.key,
                                        onChange: (e: any) => {
                                          onChangeExtraTextsField(
                                            index,
                                            idx,
                                            'key',
                                            e.target.value,
                                          );
                                        },
                                      }}
                                    />
                                  </div>
                                  <div className="text-value">
                                    <ProFormText
                                      name={[index, 'value', idx, 'value']}
                                      fieldProps={{
                                        value: inner.value,
                                        onChange: (e: any) => {
                                          onChangeExtraTextsField(
                                            index,
                                            idx,
                                            'value',
                                            e.target.value,
                                          );
                                        },
                                      }}
                                    />
                                  </div>
                                  <div className="text-action">
                                    <Tag
                                      onClick={() =>
                                        onMoveUpExtraTextsField(index, idx)
                                      }
                                    >
                                      <UpOutlined />
                                    </Tag>
                                    <Tag
                                      onClick={() =>
                                        onMoveDownExtraTextsField(index, idx)
                                      }
                                    >
                                      <DownOutlined />
                                    </Tag>
                                    <Tag
                                      color="red"
                                      onClick={() =>
                                        onRemoveExtraTextsField(index, idx)
                                      }
                                    >
                                      <DeleteOutlined />
                                    </Tag>
                                  </div>
                                </div>
                              ))
                            : null}
                          <div className="text-group">
                            <div className="text-key">
                              <Tag
                                color="blue"
                                className="add-line"
                                onClick={() => onAddExtraTextsField(index)}
                              >
                                {intl.formatMessage({
                                  id: 'content.param.add-line',
                                })}
                              </Tag>
                            </div>
                          </div>
                        </div>
                      </ProFormText>
                    ) : item.type === 'archive' ? (
                      <ProFormText
                        width="lg"
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                      >
                        <ProFormSelect
                          name={[index, 'value']}
                          showSearch
                          mode="multiple"
                          options={searchArchives.map((a: any) => ({
                            title: a.title,
                            label: a.title,
                            value: a.id,
                          }))}
                          fieldProps={{
                            value: item.value || undefined,
                            onChange: (e: any) => {
                              handleUpdateFieldValue(index, e);
                            },
                            onSearch: (e) => {
                              onSearchArchives(e);
                            },
                          }}
                        />
                      </ProFormText>
                    ) : item.type === 'category' ? (
                      <ProFormText
                        width="lg"
                        label={
                          <span>
                            {item.remark} (
                            {intl.formatMessage({
                              id: 'setting.system.param-name',
                            })}
                            :{item.name})
                          </span>
                        }
                      >
                        <ProFormSelect
                          showSearch
                          name={[index, 'value']}
                          mode={'single'}
                          options={[
                            {
                              title: intl.formatMessage({
                                id: 'content.please-select',
                              }),
                              value: 0,
                              status: 1,
                            },
                          ]
                            .concat(categories)
                            .map((cat: any) => ({
                              title: cat.title,
                              label: (
                                <div title={cat.title}>
                                  {cat.parents?.length > 0 ? (
                                    <span className="text-muted">
                                      {cat.parents
                                        ?.map((parent: any) => parent.title)
                                        .join(' > ')}
                                      {' > '}
                                    </span>
                                  ) : (
                                    ''
                                  )}
                                  {cat.title}
                                </div>
                              ),
                              value: cat.id,
                              disabled: cat.status !== 1,
                            }))}
                          fieldProps={{
                            showSearch: true,
                            filterOption: (input: string, option: any) =>
                              (option?.title ?? option?.label)
                                .toLowerCase()
                                .includes(input.toLowerCase()),
                            value: Number(item.value) || undefined,
                            onChange: (e: any) => {
                              handleUpdateFieldValue(index, e);
                            },
                          }}
                        />
                      </ProFormText>
                    ) : (
                      ''
                    )}
                  </Col>
                ),
            )}
          </Row>
        </ProForm>
      </Card>
      {fieldsOpen && (
        <Modal
          width={800}
          open={fieldsOpen}
          onCancel={() => {
            setFieldsOpen(false);
          }}
          footer={null}
        >
          <ProTable<any>
            headerTitle={intl.formatMessage({
              id: 'setting.system.diy-params',
            })}
            key="fields-table"
            rowKey="name"
            search={false}
            actionRef={actionRef}
            toolBarRender={() => [
              <Button
                key="add"
                type="primary"
                onClick={() => {
                  handleEditField(-1);
                }}
              >
                <FormattedMessage id="content.module.field.add" />
              </Button>,
            ]}
            tableAlertRender={false}
            tableAlertOptionRender={false}
            request={async () => {
              return {
                data: setting || [],
                success: true,
              };
            }}
            columns={columns}
            pagination={false}
          />
        </Modal>
      )}
      {addFieldOpen && (
        <ModalForm
          width={600}
          title={
            currentField.name
              ? currentField.name +
                intl.formatMessage({ id: 'content.module.field.edit' })
              : intl.formatMessage({ id: 'content.module.field.add' })
          }
          open={addFieldOpen}
          modalProps={{
            onCancel: () => {
              setAddFieldOpen(false);
            },
          }}
          initialValues={currentField}
          layout="horizontal"
          onFinish={async (values) => {
            handleSaveField(values);
          }}
        >
          <ProFormText
            name="remark"
            label={intl.formatMessage({
              id: 'setting.system.remark',
            })}
          />
          <ProFormText
            name="name"
            required
            label={intl.formatMessage({
              id: 'setting.system.param-name',
            })}
            disabled={currentField.name ? true : false}
            extra={intl.formatMessage({
              id: 'setting.system.param-name-description',
            })}
          />
          <ProFormSelect
            name="group"
            label={intl.formatMessage({ id: 'content.module.field.group' })}
            options={groupNames.map((name: string) => ({
              label: name || intl.formatMessage({ id: 'design.diyfield.ungrouped' }),
              value: name,
            }))}
            fieldProps={{
              popupRender: (menu) => (
                <>
                  {menu}
                  <Divider style={{ margin: '8px 0' }} />
                  <Space style={{ padding: '0 8px 4px' }}>
                    <Input
                      placeholder={intl.formatMessage({
                        id: 'setting.banner.group.placeholder',
                      })}
                      ref={inputRef}
                      value={groupName}
                      onChange={onGroupNameChange}
                      onKeyDown={(e) => e.stopPropagation()}
                    />
                    <Button
                      type="text"
                      icon={<PlusOutlined />}
                      onClick={addGroup}
                    >
                      {intl.formatMessage({ id: 'setting.banner.group.add' })}
                    </Button>
                  </Space>
                </>
              ),
            }}
          />
          <ProFormRadio.Group
            name="type"
            label={intl.formatMessage({ id: 'content.module.field.type' })}
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
            }}
          />
          <ProFormTextArea
            label={intl.formatMessage({ id: 'content.category.default' })}
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
    </NewContainer>
  );
};

export default SettingDiyFieldFrom;
