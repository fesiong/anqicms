import NewContainer from '@/components/NewContainer';
import AttachmentSelect from '@/components/attachment';
import {
  deleteSettingNav,
  getArchives,
  getCategories,
  getModules,
  getSettingNav,
  getSettingNavTypes,
  saveSettingNav,
} from '@/services';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProColumns,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Image, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import NavTypes from './components/navType';

const SettingNavFrom: React.FC<any> = () => {
  const [navs, setNavList] = useState<any>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [modules, setModules] = useState<any[]>([]);
  const [editingNav, setEditingNav] = useState<any>({ nav_type: 0 });
  const [modalVisible, setModalVisible] = useState<boolean>(false);
  const [nav_type, setNavType] = useState<number>(0);
  const [innerOptions, setInnerOptions] = useState<any[]>([]);
  const [navTypes, setNavTypes] = useState<any[]>([]);
  const [typeId, setTypeId] = useState<number>(1);
  const [archives, setArchives] = useState<any[]>([]);
  const [defaultTitle, setDefaultTitle] = useState<string>('');
  const [navLogo, setNavLogo] = useState<string>('');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getNavList = async (typeId: number) => {
    const res = await getSettingNav({
      type_id: typeId,
    });
    let navs = res.data || [];
    setNavList(navs);
  };

  const getNavTypes = async () => {
    const res = await getSettingNavTypes();
    let types = res.data || [];
    setNavTypes(types);
  };

  const getCategoryList = async () => {
    const res = await getCategories();
    const pages = await getCategories({ type: 3 });
    let categories = (res.data || []).concat(pages.data || []);
    setCategories(categories);
  };

  const getModuleList = async () => {
    const res = await getModules();
    let modules = res.data || [];
    setModules(modules);
    let options = [
      {
        value: 0,
        label: intl.formatMessage({ id: 'setting.nav.home' }),
      },
    ];

    for (let item of modules) {
      options.push({
        value: item.id,
        label: item.name + intl.formatMessage({ id: 'setting.nav.home' }),
      });
    }

    setInnerOptions(options);
  };

  const onTabChange = (key: string) => {
    // 重复执行useEffect
    getNavList(typeId);
    getCategoryList();
    getModuleList();
    getNavTypes();

    setNewKey(key);
  };

  useEffect(() => {
    getNavList(typeId);
    getCategoryList();
    getModuleList();
    getNavTypes();
  }, []);

  const editNav = (row: any) => {
    setEditingNav(row);
    setNavLogo(row.logo);
    setNavType(row.nav_type);
    setModalVisible(true);
  };

  const removeNav = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.nav.confirm-delete' }),
      onOk: () => {
        deleteSettingNav(row).then((res) => {
          message.success(res.msg);
          getNavList(typeId);
        });
      },
    });
  };

  const handleShowAddNav = () => {
    setNavType(0);
    setNavLogo('');
    setEditingNav({ nav_type: 0 });
    setModalVisible(true);
  };

  const onNavSubmit = async (data: any) => {
    let values = Object.assign(editingNav, data);
    values.type_id = typeId;
    if (!values.title) {
      values.title = defaultTitle;
    }
    values.logo = navLogo;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingNav(values)
      .then((res) => {
        message.success(res.msg);
        setModalVisible(false);
        getNavList(typeId);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const getModuleName = (moduleId: number) => {
    for (let item of modules) {
      if (item.id === moduleId) {
        return item.name;
      }
    }
  };

  const onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      setArchives(res.data || []);
    });
  };

  const handleChangeNavType = (typeId: number) => {
    setTypeId(typeId);
    getNavList(typeId);
  };

  const handleSelectLogo = (row: any) => {
    setNavLogo(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleCleanLogo = (e: any) => {
    e.stopPropagation();
    setNavLogo('');
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'setting.nav.title' }),
      dataIndex: 'title',
      render: (text, record) => {
        return (
          <div>
            <span>{text}</span>
            {record.sub_title && (
              <span className="text-muted"> - {record.sub_title}</span>
            )}
          </div>
        );
      },
    },
    {
      title:
        intl.formatMessage({ id: 'setting.nav.type' }) +
        '/' +
        intl.formatMessage({ id: 'setting.nav.description' }),
      dataIndex: 'page_id',
      render: (_, record) => {
        return (
          <Space size={8} direction="vertical">
            <div>
              <a href={record.link} target="_blank" rel="noreferrer">
                <Tag color="blue">
                  {record.nav_type === 2
                    ? intl.formatMessage({ id: 'setting.nav.outlink' }) +
                      ': ' +
                      record.link
                    : record.nav_type === 3
                    ? intl.formatMessage({ id: 'setting.nav.archive' }) +
                      ': ' +
                      record.page_id
                    : record.nav_type === 1
                    ? intl.formatMessage({ id: 'setting.nav.category' }) +
                      ': ' +
                      record.page_id
                    : intl.formatMessage({ id: 'setting.nav.internal' }) +
                      ': ' +
                      (record.page_id > 0
                        ? getModuleName(record.page_id)
                        : intl.formatMessage({ id: 'setting.nav.home' }))}
                </Tag>
              </a>
            </div>
            {record.description && (
              <div className="text-muted">{record.description}</div>
            )}
          </Space>
        );
      },
    },
    {
      title: 'thumb',
      dataIndex: 'thumb',
      hideInSearch: true,
      width: 70,
      render: (text, record) => {
        return text ? <img src={record.thumb} className="list-thumb" /> : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          {(record.parent_id === 0 ||
            record.level < 2 ||
            record.nav_list?.length > 0) && (
            <a onClick={() => editNav({ parent_id: record.id })} key="add">
              <FormattedMessage id="setting.nav.children.add" />
            </a>
          )}
          <a onClick={() => editNav(record)} key="edit">
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a onClick={() => removeNav(record)} key="warning">
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        activeTabKey={typeId + ''}
        onTabChange={(tabKey) => handleChangeNavType(Number(tabKey))}
        tabList={navTypes.map((item) => ({
          key: item.id + '',
          label: item.title,
        }))}
      >
        <ProTable<any>
          key={newKey}
          headerTitle={intl.formatMessage({ id: 'setting.nav.list' })}
          dataSource={navs}
          rowKey="id"
          search={false}
          toolBarRender={() => [
            <Button key="add" onClick={handleShowAddNav}>
              <FormattedMessage id="setting.nav.add" />
            </Button>,
            <NavTypes
              key="type"
              onCancel={() => {
                getNavTypes();
              }}
            >
              <Button>
                <FormattedMessage id="setting.nav.types" />
              </Button>
            </NavTypes>,
          ]}
          columns={columns}
          expandable={{
            childrenColumnName: 'nav_list',
            defaultExpandAllRows: true,
          }}
          pagination={false}
        />
      </Card>
      {modalVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'menu.setting.nav' })}
          open={modalVisible}
          modalProps={{
            onCancel: () => setModalVisible(false),
          }}
          initialValues={editingNav}
          onFinish={onNavSubmit}
        >
          <ProFormSelect
            name="parent_id"
            width="lg"
            label={intl.formatMessage({ id: 'setting.nav.parent' })}
            fieldProps={{
              fieldNames: {
                label: 'title',
                value: 'id',
              },
              optionItemRender(item: any) {
                return (
                  <div title={item.title}>
                    {item.parents?.length > 0 ? (
                      <span className="text-muted">
                        {item.parents
                          ?.map((parent: any) => parent.title)
                          .join(' > ')}
                        {' > '}
                      </span>
                    ) : (
                      ''
                    )}
                    {item.title}
                  </div>
                );
              },
            }}
            request={async () => {
              let newNavs = [
                {
                  parents: [],
                  title: intl.formatMessage({ id: 'setting.nav.top' }),
                  id: 0,
                },
              ];
              for (let item of navs || []) {
                newNavs.push({
                  parents: [],
                  title: item.title,
                  id: item.id,
                });
                for (let sub of item.nav_list || []) {
                  let subItem = {
                    parents: sub.parents || [],
                    title: sub.title,
                    id: sub.id,
                  };
                  newNavs.push(subItem);
                  for (let sub2 of sub.nav_list || []) {
                    newNavs.push({
                      parents: sub2.parents || [],
                      title: sub2.title,
                      id: sub2.id,
                    });
                  }
                }
              }
              return newNavs;
            }}
          />
          <ProFormText
            name="title"
            label={intl.formatMessage({ id: 'setting.nav.title' })}
            width="lg"
            extra={intl.formatMessage({ id: 'setting.nav.title.description' })}
          />
          <ProFormText
            name="sub_title"
            label={intl.formatMessage({ id: 'setting.nav.subtitle' })}
            width="lg"
            extra={intl.formatMessage({
              id: 'setting.nav.subtitle.description',
            })}
          />
          <ProFormText
            name="description"
            label={intl.formatMessage({ id: 'setting.nav.description' })}
            width="lg"
          />
          <ProFormText
            width="lg"
            label={intl.formatMessage({ id: 'content.category.thumb' })}
          >
            {navLogo ? (
              <div className="ant-upload-item">
                <Image
                  preview={{
                    src: navLogo,
                  }}
                  src={navLogo}
                />
                <span className="delete" onClick={handleCleanLogo}>
                  <DeleteOutlined />
                </span>
              </div>
            ) : (
              <AttachmentSelect onSelect={handleSelectLogo} open={false}>
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
          <ProFormRadio.Group
            name="nav_type"
            label={intl.formatMessage({ id: 'setting.nav.type' })}
            fieldProps={{
              onChange: (e: any) => {
                setNavType(e.target.value);
              },
            }}
            options={[
              {
                value: 0,
                label: intl.formatMessage({ id: 'setting.nav.internal' }),
              },
              {
                value: 1,
                label: intl.formatMessage({ id: 'setting.nav.category' }),
              },
              {
                value: 3,
                label: intl.formatMessage({ id: 'setting.nav.archive' }),
              },
              {
                value: 2,
                label: intl.formatMessage({ id: 'setting.nav.outlink' }),
              },
            ]}
          />
          {nav_type === 0 && (
            <ProFormRadio.Group
              name="page_id"
              label={intl.formatMessage({ id: 'setting.nav.internal.name' })}
              options={innerOptions}
            />
          )}
          {nav_type === 1 && (
            <ProFormSelect
              name="page_id"
              width="lg"
              showSearch
              label={intl.formatMessage({ id: 'setting.nav.select-page' })}
              options={categories.map((cat: any) => ({
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
            />
          )}
          {nav_type === 3 && (
            <ProFormSelect
              name="page_id"
              width="lg"
              label={intl.formatMessage({ id: 'setting.nav.select-archive' })}
              showSearch
              options={archives.map((a: any) => ({
                title: a.title,
                label: a.title,
                value: a.id,
              }))}
              fieldProps={{
                onSearch: (e) => {
                  onSearchArchives(e);
                },
                onChange: (_, a: any) => {
                  setDefaultTitle(a.title);
                },
              }}
            />
          )}
          {nav_type === 2 && (
            <ProFormText
              name="link"
              label={intl.formatMessage({ id: 'setting.nav.link' })}
              width="lg"
              extra={intl.formatMessage({ id: 'setting.nav.link.description' })}
            />
          )}
          <ProFormText
            name="style"
            label={intl.formatMessage({ id: 'setting.nav.style' })}
            width="lg"
            extra={intl.formatMessage({ id: 'setting.nav.style.description' })}
          />
          <ProFormDigit
            name="sort"
            label={intl.formatMessage({ id: 'setting.nav.sort' })}
            width="lg"
            extra={intl.formatMessage({ id: 'setting.nav.sort.description' })}
          />
        </ModalForm>
      )}
    </NewContainer>
  );
};

export default SettingNavFrom;
