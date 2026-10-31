import { useVipModal } from '@/components/vipModal';
import {
  deleteWebsiteInfo,
  getSiteInfo,
  getSubsiteAdminLoginUrl,
  getWebsiteInfo,
  getWebsiteList,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Button, Checkbox, Modal, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import WebsiteForm from './components/form';

let submiting = false;
const WebsiteList: React.FC = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const { initialState } = useModel('@@initialState');
  const actionRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [editInfo, setEditInfo] = useState<any>({});
  const [siteInfo, setSiteInfo] = useState<any>({});
  const [multiSites, setMultiSites] = useState<any[]>([]);
  const [defaultSite, setDefaultSite] = useState<any>({});
  const inputRef = useRef<any>();
  const intl = useIntl();

  const initSiteInfo = async () => {
    getSiteInfo({}).then((res) => {
      setSiteInfo(res?.data || {});
    });
  };

  useEffect(() => {
    initSiteInfo();
  }, []);

  const handleEdit = (record: any) => {
    if (record.id > 0) {
      getWebsiteInfo({ id: record.id }).then((res) => {
        setEditInfo(res.data);
        setEditVisible(true);
      });
    } else {
      setEditInfo(record);
      setEditVisible(true);
    }
  };

  const visitSystem = (record: any) => {
    getSubsiteAdminLoginUrl({ site_id: record.id }).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
      } else {
        window.open(res.data);
      }
    });
  };

  const handleRemove = (record: any) => {
    if (record.id === 1) {
      message.error(intl.formatMessage({ id: 'website.cannot-delete' }));
      return;
    }
    if (submiting) {
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      content: (
        <div>
          <div className="mb-normal">
            {intl.formatMessage({ id: 'website.delete.tips' })}
          </div>
          <div style={{ padding: '10px 0' }}>
            <div>
              <Checkbox ref={inputRef} value={true}>
                {intl.formatMessage({ id: 'website.delete.all' })}
              </Checkbox>
            </div>
          </div>
        </div>
      ),
      onOk: async () => {
        submiting = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'website.delete.deleting' }),
          0,
        );
        const removeFile = inputRef.current?.input?.checked || false;
        deleteWebsiteInfo({
          id: record.id,
          remove_file: removeFile,
        })
          .then((res) => {
            message.info(res.msg);
            actionRef.current?.reload();
          })
          .finally(() => {
            submiting = false;
            hide();
          });
      },
    });
  };

  const onSubmit = async () => {
    setEditVisible(false);
    actionRef.current?.reload();
  };

  const handleAddSite = () => {
    if (multiSites.length > 4) {
      checkVip(() => {
        handleEdit({
          mysql: { use_default: true },
          initialed: false,
          status: 1,
        });
      }, intl.formatMessage({ id: 'website.vip.multi-site' }));
    } else {
      handleEdit({
        mysql: { use_default: true },
        initialed: false,
        status: 1,
      });
    }
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      hideInSearch: true,
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'setting.system.site-name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'setting.system.base-url' }),
      dataIndex: 'base_url',
      render: (text) => (
        <a href={text as string} target="_blank" rel="noreferrer">
          {text}
        </a>
      ),
    },
    {
      title: intl.formatMessage({ id: 'website.root-path' }),
      hideInSearch: true,
      dataIndex: 'root_path',
    },
    {
      title: intl.formatMessage({ id: 'website.create-time' }),
      hideInSearch: true,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      hideInSearch: true,
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'setting.content.notenable' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'setting.content.enable' }),
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
          {record.id === siteInfo.id ? (
            <Tag>
              <FormattedMessage id="website.current" />
            </Tag>
          ) : (
            <a
              key="edit"
              onClick={() => {
                visitSystem(record);
              }}
            >
              <FormattedMessage id="website.visit-backend" />
            </a>
          )}
          {initialState?.currentUser?.site_id === 1 && (
            <>
              <a
                key="edit"
                onClick={() => {
                  handleEdit(record);
                }}
              >
                <FormattedMessage id="setting.action.edit" />
              </a>
              {record.id === 1 ? (
                <Tag>
                  <FormattedMessage id="website.default" />
                </Tag>
              ) : (
                <a
                  className="text-red"
                  key="delete"
                  onClick={async () => {
                    await handleRemove(record);
                  }}
                >
                  <FormattedMessage id="setting.system.delete" />
                </a>
              )}
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'website.list' })}
        rowKey="id"
        actionRef={actionRef}
        request={async (params) => {
          const res = await getWebsiteList(params);
          setMultiSites(res.data || []);
          setDefaultSite(res.data?.[0] || {});
          return res;
        }}
        columnsState={{
          persistenceKey: 'website-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        toolBarRender={() => [
          initialState?.currentUser?.site_id === 1 && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                handleAddSite();
              }}
            >
              <PlusOutlined /> <FormattedMessage id="website.add" />
            </Button>
          ),
        ]}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
      {editVisible && (
        <WebsiteForm
          onCancel={() => setEditVisible(false)}
          onSubmit={onSubmit}
          open={editVisible}
          website={editInfo}
          rootPath={defaultSite.root_path}
        />
      )}
      <VipModal />
    </PageContainer>
  );
};

export default WebsiteList;
