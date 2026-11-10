import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import { deleteCategory } from '@/services';
import { getPlaces, getPlaceSetting } from '@/services/place';
import {
  ActionType,
  ProColumns,
  ProFormInstance,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Empty, message, Modal, Space } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import MultiPlaceAdd from './components/multi';
import PlaceSetting from './components/setting';
import './index.less';

let lastParams: any = {};

const PlaceList: React.FC = () => {
  const { checkVip, VipModal } = useVipModal();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>({});
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentPlace, setCurrentPlace] = useState<any>({});
  const [firstFetch, setFirstFetch] = useState<boolean>(false);
  const [settingVisible, setSettingVisible] = useState<boolean>(false);
  const [multiVisible, setMultiVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getPlaceSetting({});
    let setting = res.data || { fields: [] };
    setSetting(setting);
  };

  const onTabChange = (key: string, isSubSite: boolean) => {
    getSetting();
    setNewKey(key);
    setIsSubSite(isSubSite);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const beforeSearch = (searchParams: any) => {
    let params = searchParams;
    if (!firstFetch) {
      setFirstFetch(true);
      const searchParams = new URLSearchParams(window.location.search);
      lastParams.parent_id = Number(searchParams.get('parent_id') || 0);
      lastParams.status = searchParams.get('status') || 'ok';
      formRef.current?.setFieldsValue(lastParams);
      params = lastParams;
    } else {
      lastParams = params;
    }

    return params;
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.place.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteCategory({
              id: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditPlace = async (record: any) => {
    history.push(
      '/archive/place/detail?id=' +
        (record.id || 'new') +
        '&parent_id=' +
        record.parent_id,
    );
  };

  const handleAddPlace = () => {
    checkVip(() => {
      handleEditPlace({});
    }, intl.formatMessage({ id: 'content.place.vip.city' }));
  };

  const handleAddMultiPlace = async (record: any) => {
    checkVip(() => {
      setCurrentPlace(record);
      setMultiVisible(true);
    }, intl.formatMessage({ id: 'content.place.vip.city' }));
  };

  const handleShowSetting = () => {
    checkVip(() => {
      setSettingVisible(true);
    }, intl.formatMessage({ id: 'content.place.vip.city' }));
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.sort.name' }),
      dataIndex: 'sort',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.place.title' }),
      dataIndex: 'title',
      //hideInSearch: true,
      filters: true,
      render: (dom, entity) => {
        return (
          <>
            {entity.spacer}
            <a href={entity.link} target="_blank" rel="noreferrer">
              {dom}
            </a>
          </>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.place.template' }),
      dataIndex: 'template',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.category.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.category.status.hide' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'content.category.status.ok' }),
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
          {!isSubSite && (
            <>
              <a
                key="add-child"
                onClick={() => {
                  handleEditPlace({
                    parent_id: record.id,
                    status: 1,
                  });
                }}
              >
                <FormattedMessage id="content.place.add-children" />
              </a>
              <a
                key="add"
                onClick={() => {
                  handleAddMultiPlace({
                    parent_id: record.id,
                    status: 1,
                  });
                }}
              >
                <FormattedMessage id="content.place.batch-add-children" />
              </a>
            </>
          )}
          <a
            key="edit"
            onClick={() => {
              handleEditPlace(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={onTabChange}>
      <ProTable<any>
        key={newKey}
        headerTitle={intl.formatMessage({ id: 'menu.archive.place' })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          span: { xs: 24, sm: 12, md: 8, lg: 8, xl: 8, xxl: 8 },
          defaultCollapsed: false,
        }}
        formRef={formRef}
        form={{
          initialValues: lastParams,
        }}
        beforeSearchSubmit={beforeSearch}
        toolBarRender={() => [
          <Button
            type="default"
            key="fields"
            onClick={() => {
              handleShowSetting();
            }}
          >
            <FormattedMessage id="content.place.setting" />
          </Button>,
          !isSubSite && (
            <Button
              type="default"
              key="add2"
              onClick={() => {
                handleAddMultiPlace({
                  parent_id: 0,
                  status: 1,
                });
              }}
            >
              <FormattedMessage id="content.place.batch-add" />
            </Button>
          ),
          !isSubSite && (
            <Button
              type="default"
              key="add"
              onClick={() => {
                handleAddPlace();
              }}
            >
              <FormattedMessage id="content.place.add" />
            </Button>
          ),
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
            <Button
              size={'small'}
              onClick={() => {
                handleRemove(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-delete" />
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={async (params) => {
          if (!setting.open) {
            return [];
          }
          lastParams = params;
          return getPlaces(params);
        }}
        locale={
          !setting.open
            ? {
                emptyText: (
                  <Empty
                    description={intl.formatMessage({
                      id: 'content.place.not-open',
                    })}
                  >
                    <Button
                      type="default"
                      key="fields"
                      onClick={() => {
                        handleShowSetting();
                      }}
                    >
                      <FormattedMessage id="content.place.setting" />
                    </Button>
                  </Empty>
                ),
              }
            : {}
        }
        columnsState={{
          persistenceKey: 'place-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
          defaultCurrent: lastParams.current,
          defaultPageSize: lastParams.pageSize,
        }}
      />
      {settingVisible && (
        <PlaceSetting
          open={settingVisible}
          onCancel={() => setSettingVisible(false)}
          onSubmit={async () => {
            getSetting();
            setSettingVisible(false);
            actionRef.current?.reloadAndRest?.();
          }}
        />
      )}
      {multiVisible && (
        <MultiPlaceAdd
          open={multiVisible}
          onCancel={() => setMultiVisible(false)}
          onSubmit={async () => {
            setSelectedRowKeys([]);
            setMultiVisible(false);
            actionRef.current?.reloadAndRest?.();
          }}
          place={currentPlace}
        />
      )}
      <VipModal />
    </NewContainer>
  );
};

export default PlaceList;
