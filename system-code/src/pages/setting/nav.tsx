import React, { useEffect, useState } from 'react';
import {
  ModalForm,
  ProFormText,
  ProFormSelect,
  ProFormRadio,
  ProFormDigit,
} from '@ant-design/pro-form';
import ProList from '@ant-design/pro-list';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Button, Card, message, Modal, Space, Tag } from 'antd';
import { deleteSettingNav, getSettingNav, saveSettingNav, getCategories, getModules } from '@/services';

const SettingNavFrom: React.FC<any> = (props) => {
  const [navs, setNavList] = useState<any>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [modules, setModules] = useState<any[]>([]);
  const [editingNav, setEditingNav] = useState<any>({ nav_type: 0 });
  const [modalVisible, setModalVisible] = useState<boolean>(false);
  const [nav_type, setNavType] = useState<number>(0);
  const [innerOptions, setInnerOptions] = useState<any[]>([]);

  useEffect(() => {
    getNavList();
    getCategoryList();
    getModuleList();
  }, []);

  const getNavList = async () => {
    const res = await getSettingNav();
    let navs = res.data || [];
    setNavList(navs);
  };

  const getCategoryList = async () => {
    const res = await getCategories();
    let categories = res.data || [];
    setCategories(categories);
  };

  const getModuleList = async () => {
    const res = await getModules();
    let modules = res.data || [];
    setModules(modules);
    let options = [{
      value: 0,
      label: '首页',
    }];

    for (let item of modules) {
      options.push({
        value: item.id,
        label: item.title + '首页',
      })
    }

    setInnerOptions(options)
  }

  const editNav = (row: any) => {
    setEditingNav(row);
    setNavType(row.nav_type);
    setModalVisible(true);
  };

  const removeNav = (row: any) => {
    Modal.confirm({
      title: '确定要删除该导航吗',
      onOk: () => {
        deleteSettingNav(row).then(res => {
          message.success(res.msg);
          getNavList();
        })
      }
    })
  };

  const handleShowAddNav = () => {
    setNavType(0);
    setEditingNav({ nav_type: 0 });
    setModalVisible(true);
  };

  const onNavSubmit = async (values: any) => {
    values = Object.assign(editingNav, values)
    saveSettingNav(values)
      .then((res) => {
        message.success(res.msg);
        setModalVisible(false);
        getNavList();
      })
      .catch((err) => {
        console.log(err);
      });
  };

  const getModuleName = (moduleId: number) => {
    for (let item of modules) {
      if (item.id == moduleId) {
        return item.title;
      }
    }
  }

  return (
    <PageHeaderWrapper>
      <Card>
        <ProList<any>
          toolBarRender={() => {
            return [<Button onClick={handleShowAddNav}>添加导航</Button>];
          }}
          rowKey="name"
          headerTitle="导航列表"
          dataSource={navs}
          showActions="hover"
          showExtra="hover"
          metas={{
            title: {
              render: (text: any, row: any) => {
                return (row.parent_id > 0 ? '└  ' : '') + text;
              },
              dataIndex: 'title',
            },
            description: {
              dataIndex: 'description',
            },
            subTitle: {
              render: (text: any, row: any) => {
                return (
                  <Space size={0}>
                    {row.sub_title && <Tag>{row.sub_title}</Tag>}
                    <Tag color="blue">
                      {row.nav_type == 2
                        ? '外链: ' + row.link
                        : row.nav_type == 1
                        ? '分类/页面: ' + row.page_id
                        : '内置: ' +
                          (row.page_id > 0
                            ? getModuleName(row.page_id)
                            : '首页')}
                    </Tag>
                  </Space>
                );
              },
            },
            actions: {
              render: (text: any, row: any) => [
                <a onClick={() => editNav(row)} key="link">
                  编辑
                </a>,
                <a onClick={() => removeNav(row)} key="warning">
                  删除
                </a>,
              ],
            },
          }}
        />
      </Card>
      {modalVisible && (
        <ModalForm
          title="导航设置"
          visible={modalVisible}
          modalProps={{
            onCancel: () => setModalVisible(false),
          }}
          initialValues={editingNav}
          onFinish={onNavSubmit}
        >
          <ProFormSelect
            name="parent_id"
            width="lg"
            label="上级导航"
            fieldProps={{
              fieldNames: {
                label: 'title',
                value: 'id',
              },
            }}
            request={async () => {
              let newNavs = [
                {
                  title: '顶级导航',
                  id: 0,
                },
              ]
              for (let item of navs) {
                if (item.parent_id == 0) {
                  newNavs.push(item);
                }
              }
              return newNavs;
            }}
          />
          <ProFormText name="title" label="显示名称" width="lg" extra="在导航上显示的名称" />
          <ProFormText name="sub_title" label="子标题名称" width="lg" extra="导航名称下方的小字" />
          <ProFormText name="description" label="导航描述" width="lg" />
          <ProFormRadio.Group
            name="nav_type"
            label="模板类型"
            fieldProps={{
              onChange: (e: any) => {
                setNavType(e.target.value);
              },
            }}
            options={[
              {
                value: 0,
                label: '内置',
              },
              {
                value: 1,
                label: '分类/页面',
              },
              {
                value: 2,
                label: '外链',
              },
            ]}
          />
          {nav_type == 0 && (
            <ProFormRadio.Group
              name="page_id"
              label="内置导航"
              options={innerOptions}
            />
          )}
          {nav_type == 1 && (
            <ProFormSelect
              name="page_id"
              width="lg"
              label="选择分类/页面"
              options={categories}
              fieldProps={{
                fieldNames: {
                  label: 'title',
                  value: 'id',
                },
                optionItemRender(item) {
                  return <div dangerouslySetInnerHTML={{ __html: item.spacer + item.title }}></div>;
                },
              }}
            />
          )}
          {nav_type == 2 && (
            <ProFormText
              name="link"
              label="填写链接"
              width="lg"
              extra="连接使用http或https开头，如： https://www.kandaoni.com/"
            />
          )}
          <ProFormDigit
            name="sort"
            label="显示顺序"
            width="lg"
            extra="值越小，排序越靠前，默认99"
          />
        </ModalForm>
      )}
    </PageHeaderWrapper>
  );
};

export default SettingNavFrom;
