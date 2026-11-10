import AttachmentSelect from '@/components/attachment';
import NewContainer from '@/components/NewContainer';
import {
  deleteSettingBanner,
  getSettingBanners,
  saveSettingBanner,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Button,
  Card,
  Divider,
  Image,
  Input,
  InputRef,
  message,
  Modal,
  Space,
} from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const SettingBannerFrom: React.FC<any> = () => {
  const actionRef = useRef<ActionType>();
  const [editingBanner, setEditingBanner] = useState<any>({});
  const [modalVisible, setModalVisible] = useState<boolean>(false);
  const [banners, setBanners] = useState<any[]>([]);
  const [currentType, setCurrentType] = useState<string>('default');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const [name, setName] = useState('');
  const inputRef = useRef<InputRef>(null);

  const onNameChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setName(event.target.value);
  };

  const addItem = (
    e: React.MouseEvent<HTMLButtonElement | HTMLAnchorElement>,
  ) => {
    e.preventDefault();
    if (!name) {
      message.error(intl.formatMessage({ id: 'setting.banner.name.require' }));
      return;
    }
    setBanners([...banners, { type: name, list: [] }]);
    setName('');
    setTimeout(() => {
      inputRef.current?.focus();
    }, 0);
  };

  const getBanners = async () => {
    getSettingBanners().then((res) => {
      let data = res.data || [];
      setBanners(data);
      if (currentType === 'default' && data.length > 0) {
        setCurrentType(data[0].type);
      }
    });
  };

  const onTabChange = (key: string) => {
    getBanners().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getBanners();
  }, []);

  const editBanner = (row: any) => {
    setEditingBanner(row);
    setModalVisible(true);
  };

  const removeBanner = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.banner.confirm-delete' }),
      onOk: () => {
        deleteSettingBanner(row).then((res) => {
          message.success(res.msg);
          getBanners();
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleSelectLogo = (row: any) => {
    editingBanner.logo = row.file_path;
    setEditingBanner(Object.assign({}, editingBanner));
  };

  const handleRemoveLogo = (e: any) => {
    e.stopPropagation();
    editingBanner.logo = '';
    setEditingBanner(Object.assign({}, editingBanner));
  };

  const handleShowAdd = () => {
    setEditingBanner({});
    setModalVisible(true);
  };

  const onBannerSubmit = async (data: any) => {
    let values = Object.assign(editingBanner, data);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    if (!values.type) {
      values['type'] = 'default';
    }
    saveSettingBanner(values)
      .then((res) => {
        message.success(res.msg);
        setModalVisible(false);
        getBanners();
        actionRef.current?.reload();
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'setting.banner.logo' }),
      dataIndex: 'logo',
      width: 800,
      render: (_, record) => (
        <div className="text-center">
          <Image
            src={record.logo}
            style={{ height: 150, objectFit: 'contain' }}
          ></Image>
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.banner.name-link' }),
      dataIndex: 'link',
      width: 800,
      render: (_, record) => (
        <div>
          <div>
            <FormattedMessage id="setting.banner.link" />
            {record.link}
          </div>
          <div>
            <FormattedMessage id="setting.banner.alt" />
            {record.alt}
          </div>
          <div>
            <FormattedMessage id="setting.banner.description" />
            {record.description}
          </div>
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 180,
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              editBanner(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              removeBanner(record);
            }}
          >
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
        activeTabKey={currentType}
        onTabChange={(tabKey) => setCurrentType(tabKey)}
        tabList={banners.map((item: any) => ({
          key: item.type,
          label: item.type,
        }))}
      >
        {banners.map((item: any, index: number) =>
          item.type === currentType ? (
            <ProTable
              key={index}
              toolBarRender={() => {
                return [
                  <Button key="add" onClick={handleShowAdd}>
                    <FormattedMessage id="setting.action.add" />
                  </Button>,
                ];
              }}
              rowKey="name"
              headerTitle={
                currentType +
                intl.formatMessage({ id: 'setting.banner.group-list' })
              }
              columnsState={{
                persistenceKey: 'banner-table',
                persistenceType: 'localStorage',
              }}
              columns={columns}
              actionRef={actionRef}
              search={false}
              dataSource={item.list}
              pagination={false}
            />
          ) : null,
        )}
        {modalVisible && (
          <ModalForm
            width={600}
            title={intl.formatMessage({ id: 'setting.banner.banner' })}
            open={modalVisible}
            modalProps={{
              onCancel: () => setModalVisible(false),
            }}
            initialValues={editingBanner}
            onFinish={onBannerSubmit}
          >
            <ProFormSelect
              name="type"
              label={intl.formatMessage({ id: 'setting.banner.group' })}
              options={banners.map((a: any) => ({
                label: a.type,
                value: a.type,
              }))}
              fieldProps={{
                dropdownRender: (menu) => (
                  <>
                    {menu}
                    <Divider style={{ margin: '8px 0' }} />
                    <Space style={{ padding: '0 8px 4px' }}>
                      <Input
                        placeholder={intl.formatMessage({
                          id: 'setting.banner.group.placeholder',
                        })}
                        ref={inputRef}
                        value={name}
                        onChange={onNameChange}
                        onKeyDown={(e) => e.stopPropagation()}
                      />
                      <Button
                        type="text"
                        icon={<PlusOutlined />}
                        onClick={addItem}
                      >
                        {intl.formatMessage({ id: 'setting.banner.group.add' })}
                      </Button>
                    </Space>
                  </>
                ),
              }}
            />
            <ProFormText
              label={intl.formatMessage({ id: 'setting.banner.logo-name' })}
            >
              <AttachmentSelect onSelect={handleSelectLogo} open={false}>
                <div className="ant-upload-item">
                  {editingBanner.logo ? (
                    <>
                      <img src={editingBanner.logo} style={{ width: '100%' }} />
                      <a className="delete" onClick={handleRemoveLogo}>
                        <FormattedMessage id="setting.system.delete" />
                      </a>
                    </>
                  ) : (
                    <div className="add">
                      <PlusOutlined />
                      <div style={{ marginTop: 8 }}>
                        <FormattedMessage id="setting.system.upload" />
                      </div>
                    </div>
                  )}
                </div>
              </AttachmentSelect>
            </ProFormText>
            <ProFormText
              name="link"
              label={intl.formatMessage({ id: 'setting.banner.link-name' })}
            />
            <ProFormText
              name="alt"
              label={intl.formatMessage({ id: 'setting.banner.alt-name' })}
            />
            <ProFormText
              name="description"
              label={intl.formatMessage({
                id: 'setting.banner.description-name',
              })}
            />
          </ModalForm>
        )}
      </Card>
    </NewContainer>
  );
};

export default SettingBannerFrom;
