import AttachmentSelect from '@/components/attachment';
import { useVipModal } from '@/components/vipModal';
import WebsiteForm from '@/pages/website/components/form';
import {
  getSubsiteAdminLoginUrl,
  pluginGetMultiLangConfig,
  pluginGetMultiLangSites,
  pluginGetMultiLangSyncStatus,
  pluginGetMultiLangValidSites,
  pluginRemoveMultiLangSite,
  pluginSaveMultiLangConfig,
  pluginSaveMultiLangSite,
  pluginSyncMultiLangSiteContent,
} from '@/services';
import { getLanguageIcon, supportLanguages } from '@/utils';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProForm,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Avatar,
  Button,
  Card,
  Image,
  Modal,
  Progress,
  Space,
  Tag,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import TranslateHtmlCache from './components/htmlCache';
import TranslateHtmlLog from './components/htmlLog';
import TranslateTextLog from './components/textLog';

let running = false;
let intXhr: any = null;

const PluginMultiLang: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const formRef = React.createRef<ProFormInstance>();
  const actionRef = useRef<ActionType>();
  const [limiterSetting, setLimiterSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [currentSite, setCurrentSite] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [historyVisible, setHistoryVisible] = useState<boolean>(false);
  const [textLogVisible, setTextLogVisible] = useState<boolean>(false);
  const [cacheVisible, setCacheVisible] = useState<boolean>(false);
  const [addNewSiteVisible, setAddNewSiteVisible] = useState<boolean>(false);
  const [multiSites, setMultiSites] = useState<any[]>([]);
  const [defaultSite, setDefaultSite] = useState<any>({});
  const [langOptions, setLangOptions] = useState<any[]>([]);
  const [siteType, setSiteType] = useState<any>('multi');
  const [showType, setShowType] = useState<string>('');
  const [syncSite, setSyncSite] = useState<any>(null);
  const [syncConfirmVisible, setSyncConfirmVisible] = useState<boolean>(false);
  const [task, setTask] = useState<any>(null);
  const [activeTabKey, setActiveTabKey] = useState<string>('setting');
  // 已访问过的 tab 常驻挂载（仅切换显示），避免切换 tab 时表单被卸载丢失未保存输入
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['setting']);

  const intl = useIntl();

  const syncTask = async () => {
    pluginGetMultiLangSyncStatus().then((res) => {
      if (res.data) {
        running = true;
        setTask(res.data);
      } else {
        if (running) {
          running = false;
          actionRef.current?.reload();
        }
        clearInterval(intXhr);
        setTask(null);
      }
    });
  };

  const getSetting = async () => {
    const res = await pluginGetMultiLangConfig();
    let setting = res.data || {};
    // 需要转换数组成字符串
    setting.site_type = setting.site_type || 'multi';
    setSiteType(setting.site_type);
    setShowType(setting.type);
    setLimiterSetting(setting);
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
    setLangOptions(
      supportLanguages.map((item) => {
        return {
          label: item.label,
          value: item.value,
        };
      }),
    );
    // 进入页面的时候查询一次task
    syncTask();
    // 定时查询task
    intXhr = setInterval(() => {
      syncTask();
    }, 1000);
    return () => {
      running = false;
      clearInterval(intXhr);
    };
  }, []);

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    let setting = values;

    pluginSaveMultiLangConfig(setting)
      .then((res) => {
        message.success(res.msg);
        getSetting();
        actionRef.current?.reloadAndRest?.();
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const handleRemoveItem = (record: any) => {
    // remove site
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.multilang.remove.confirm',
      }),
      onOk: () => {
        pluginRemoveMultiLangSite(record)
          .then((res) => {
            message.success(res.msg);
            actionRef.current?.reloadAndRest?.();
          })
          .catch((err) => {
            console.log(err);
          });
      },
    });
  };

  const onSubmitSite = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );

    const postData = Object.assign(currentSite, values);
    pluginSaveMultiLangSite(postData)
      .then((res) => {
        message.success(res.msg);
        setEditVisible(false);
        actionRef.current?.reloadAndRest?.();
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const handleSyncData = (record: any) => {
    setSyncSite(record);
    setSyncConfirmVisible(true);
  };

  const syncSiteData = (focus: boolean) => {
    // sync website data
    syncSite.focus = focus;
    pluginSyncMultiLangSiteContent(syncSite)
      .then((res) => {
        message.success(res.msg);
        actionRef.current?.reloadAndRest?.();
        // 马上执行一遍
        intXhr = setInterval(() => {
          syncTask();
        }, 1000);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        setSyncConfirmVisible(false);
      });
  };

  const handleSearchLang = (val: string) => {
    // 如果没有匹配到，则才添加
    if (langOptions.length > supportLanguages.length) {
      langOptions.splice(langOptions.length - 1, 1);
    }
    if (
      !langOptions.find(
        (item) =>
          item.value.indexOf(val) !== -1 || item.label.indexOf(val) !== -1,
      )
    ) {
      langOptions.push({
        label: val,
        value: val,
      });
      setLangOptions([...langOptions]);
    }
  };

  const handleChangeLanguage = (val: string) => {
    currentSite.language = val;
    setCurrentSite({ ...currentSite });
  };

  const handleLoginAdmin = (record: any) => {
    getSubsiteAdminLoginUrl({ site_id: record.id }).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
      } else {
        window.open(res.data);
      }
    });
  };

  const handleCleanLogo = () => {
    currentSite.language_icon = '';
    setCurrentSite({ ...currentSite });
  };

  const handleSelectImage = (row: any) => {
    currentSite.language_icon = row.file_path;
    setCurrentSite({ ...currentSite });
  };

  const handleSelectSite = (val: any) => {
    if (val === 0) {
      setEditVisible(false);
      setAddNewSiteVisible(true);
    }
  };

  const onSubmitNewSite = async () => {
    setAddNewSiteVisible(false);
    setEditVisible(true);
  };

  const handleAddSite = () => {
    if (multiSites.length > 1) {
      checkVip(() => {
        setCurrentSite({});
        setEditVisible(true);
      }, intl.formatMessage({ id: 'plugin.multilang.vip-tip' }));
    } else {
      setCurrentSite({});
      setEditVisible(true);
    }
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.domain' }),
      dataIndex: 'base_url',
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.language' }),
      dataIndex: 'language',
      render: (text: any) => (
        <div>
          {supportLanguages.find((item) => item.value === text)?.label || text}
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.icon' }),
      dataIndex: 'language_icon',
      render: (_, record) => (
        <div style={{ fontSize: 24 }}>
          {record.language_icon ? (
            <img src={record.language_icon} style={{ width: 30, height: 30 }} />
          ) : (
            getLanguageIcon(record.language)
          )}
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.sync-time' }),
      dataIndex: 'sync_time',
      hidden: siteType === 'single',
      render: (_, record) => {
        return (
          <div>
            <div>
              {record.sync_time > 0
                ? dayjs(record.sync_time * 1000).format('YYYY-MM-DD HH:mm')
                : '-'}
            </div>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      render: (text: any, record) => (
        <Space size={20}>
          <a
            onClick={() => {
              setCurrentSite(record);
              setEditVisible(true);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          {!record.is_main ? (
            <>
              {siteType === 'multi' && (
                <>
                  <a
                    onClick={() => {
                      handleSyncData(record);
                    }}
                  >
                    <FormattedMessage id="setting.multilang.sync" />
                  </a>
                  <a
                    onClick={() => {
                      handleLoginAdmin(record);
                    }}
                  >
                    <FormattedMessage id="setting.multilang.login" />
                  </a>
                </>
              )}
              <a
                className="text-red"
                onClick={() => {
                  handleRemoveItem(record);
                }}
              >
                <FormattedMessage id="setting.system.delete" />
              </a>
            </>
          ) : (
            <Tag>
              <FormattedMessage id="plugin.multilang.is-main" />
            </Tag>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card
        activeTabKey={activeTabKey}
        onTabChange={(tabKey) => {
          setActiveTabKey(tabKey);
          setVisitedTabs((prev) =>
            prev.includes(tabKey) ? prev : [...prev, tabKey],
          );
        }}
        tabList={[
          {
            key: 'setting',
            label: intl.formatMessage({ id: 'plugin.multilang.setting' }),
          },
          {
            key: 'sites',
            label: intl.formatMessage({ id: 'plugin.multilang.sites' }),
          },
        ]}
      >
        {fetched && visitedTabs.includes('setting') && (
          <div style={{ display: activeTabKey === 'setting' ? 'block' : 'none' }}>
          <ProForm
            onFinish={onSubmit}
            initialValues={limiterSetting}
            formRef={formRef}
          >
            <ProFormRadio.Group
              name={'open'}
              label={intl.formatMessage({
                id: 'plugin.multilang.open.name',
              })}
              options={[
                {
                  label: intl.formatMessage({
                    id: 'plugin.multilang.open.false',
                  }),
                  value: false,
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.multilang.open.true',
                  }),
                  value: true,
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.multilang.open.description',
              })}
            />
            <ProFormRadio.Group
              name="site_type"
              label={intl.formatMessage({
                id: 'plugin.multilang.site-type',
              })}
              options={[
                {
                  value: 'multi',
                  label: intl.formatMessage({
                    id: 'plugin.multilang.site-type.domain',
                  }),
                },
                {
                  value: 'single',
                  label: intl.formatMessage({
                    id: 'plugin.multilang.site-type.direction',
                  }),
                },
              ]}
              fieldProps={{
                onChange: (e) => {
                  setSiteType(e.target.value);
                },
              }}
              extra={intl.formatMessage({
                id: 'plugin.multilang.site-type.description',
              })}
            />
            <ProFormRadio.Group
              name="type"
              label={intl.formatMessage({
                id: 'plugin.multilang.type',
              })}
              options={[
                {
                  value: 'domain',
                  label: intl.formatMessage({
                    id: 'plugin.multilang.type.domain',
                  }),
                },
                {
                  value: 'directory',
                  label: intl.formatMessage({
                    id: 'plugin.multilang.type.direction',
                  }),
                },
                {
                  value: 'same',
                  label: intl.formatMessage({
                    id: 'plugin.multilang.type.same-url',
                  }),
                },
              ]}
              fieldProps={{
                onChange: (e) => {
                  setShowType(e.target.value);
                },
              }}
              extra={intl.formatMessage({
                id: 'plugin.multilang.type.description',
              })}
            />
            {showType === 'directory' && (
              <ProFormRadio.Group
                name="show_main_dir"
                label={intl.formatMessage({
                  id: 'plugin.multilang.show_main_dir',
                })}
                options={[
                  {
                    value: false,
                    label: intl.formatMessage({
                      id: 'plugin.multilang.auto_translate.false',
                    }),
                  },
                  {
                    value: true,
                    label: intl.formatMessage({
                      id: 'plugin.multilang.auto_translate.true',
                    }),
                  },
                ]}
                extra={intl.formatMessage({
                  id: 'plugin.multilang.show_main_dir.description',
                })}
              />
            )}
            <ProFormSelect
              name="default_language"
              label={intl.formatMessage({
                id: 'plugin.multilang.default_language',
              })}
              style={{ width: '100%' }}
              options={supportLanguages.map((item) => {
                return {
                  label: item.label,
                  value: item.value,
                };
              })}
            />
            <ProFormRadio.Group
              name="auto_translate"
              label={intl.formatMessage({
                id: 'plugin.multilang.auto_translate',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.multilang.auto_translate.false',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.multilang.auto_translate.true',
                  }),
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.multilang.auto_translate.description',
              })}
            />
          </ProForm>
          </div>
        )}
        {fetched && visitedTabs.includes('sites') && (
          <div style={{ display: activeTabKey === 'sites' ? 'block' : 'none' }}>
          <ProTable<any>
            rowKey="language"
            search={false}
            actionRef={actionRef}
            toolBarRender={() => [
              siteType === 'single' && (
                <>
                  <Button
                    key="manage"
                    onClick={() => {
                      setTextLogVisible(true);
                    }}
                  >
                    <FormattedMessage id="plugin.multilang.text-log.manage" />
                  </Button>
                  <Button
                    key="cache"
                    onClick={() => {
                      setCacheVisible(true);
                    }}
                  >
                    <FormattedMessage id="plugin.multilang.translate-cache" />
                  </Button>
                  <Button
                    key="log"
                    onClick={() => {
                      setHistoryVisible(true);
                    }}
                  >
                    <FormattedMessage id="plugin.multilang.translate-log" />
                  </Button>
                </>
              ),
              limiterSetting.open && (
                <Button
                  key="add"
                  onClick={() => {
                    handleAddSite();
                  }}
                >
                  <FormattedMessage id="plugin.multilang.add" />
                </Button>
              ),
            ]}
            tableAlertRender={false}
            tableAlertOptionRender={false}
            request={async (params) => {
              const res = await pluginGetMultiLangSites(params);
              setMultiSites(res.data || []);
              setDefaultSite(res.data?.[0] || {});
              return res;
            }}
            columns={columns}
            pagination={false}
          />
          </div>
        )}
      </Card>
      {editVisible && (
        <ModalForm
          width={550}
          title={intl.formatMessage({
            id: 'plugin.multilang.edit',
          })}
          open={editVisible}
          initialValues={currentSite}
          layout="horizontal"
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={onSubmitSite}
        >
          {siteType === 'multi' && (
            <ProFormSelect
              label={intl.formatMessage({
                id: 'plugin.multilang.select',
              })}
              disabled={currentSite.is_main}
              showSearch
              name="id"
              request={async () => {
                const res = await pluginGetMultiLangValidSites({});
                const data = res.data.map((item: any) => {
                  return {
                    label: item.name + '(ID:' + item.id + ')',
                    value: item.id,
                    disabled: item.parent_id > 0 || item.status !== true,
                  };
                });
                data.push({
                  label: intl.formatMessage({
                    id: 'plugin.multilang.add',
                  }),
                  value: 0,
                });
                return data;
              }}
              fieldProps={{
                onSelect: handleSelectSite,
              }}
              extra={intl.formatMessage({
                id: 'plugin.multilang.select.description',
              })}
            />
          )}
          {siteType === 'single' && (
            <ProFormText
              name={'base_url'}
              label={intl.formatMessage({
                id: 'plugin.multilang.base_url.name',
              })}
              extra={intl.formatMessage({
                id: 'plugin.multilang.base_url.description',
              })}
            />
          )}
          <ProFormSelect
            name="language"
            label={intl.formatMessage({
              id: 'plugin.multilang.language',
            })}
            disabled={currentSite.is_main}
            style={{ width: '100%' }}
            showSearch
            fieldProps={{
              onSearch: handleSearchLang,
              onChange: handleChangeLanguage,
            }}
            options={langOptions}
          />
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.multilang.icon' })}
          >
            {currentSite.language_icon ? (
              <div className="ant-upload-item">
                <Image
                  preview={{
                    src: currentSite.language_icon,
                  }}
                  src={currentSite.language_icon}
                />
                <span className="delete" onClick={handleCleanLogo}>
                  <DeleteOutlined />
                </span>
              </div>
            ) : (
              <div className="ant-upload-item">
                <Avatar size={100} style={{ fontSize: 48 }}>
                  {getLanguageIcon(currentSite.language)}
                </Avatar>
              </div>
            )}
            <AttachmentSelect
              onSelect={handleSelectImage}
              open={false}
              multiple={false}
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
        </ModalForm>
      )}
      {addNewSiteVisible && (
        <WebsiteForm
          onCancel={() => setAddNewSiteVisible(false)}
          onSubmit={onSubmitNewSite}
          open={addNewSiteVisible}
          website={{
            status: 1,
            mysql: { use_default: true },
            initialed: false,
          }}
          rootPath={defaultSite.root_path}
        />
      )}
      {task !== null && (
        <Modal
          title={intl.formatMessage({ id: 'plugin.multilang.syncing' })}
          open={true}
          footer={null}
        >
          <div className="task-progress">
            <Progress percent={task.finished ? 100 : task.percent}>
              {task.finish_count}/{task.total_count}
            </Progress>
          </div>
          <div className="task-message">{task.message}</div>
        </Modal>
      )}
      {historyVisible && (
        <TranslateHtmlLog
          open={historyVisible}
          onCancel={() => setHistoryVisible(false)}
        />
      )}
      {cacheVisible && (
        <TranslateHtmlCache
          open={cacheVisible}
          onCancel={() => setCacheVisible(false)}
        />
      )}
      <Modal
        open={syncConfirmVisible}
        title={intl.formatMessage({
          id: 'plugin.multilang.sync.confirm',
        })}
        onCancel={() => setSyncConfirmVisible(false)}
        footer={[
          <Button key="cancel" onClick={() => setSyncConfirmVisible(false)}>
            <FormattedMessage id="plugin.multilang.sync.cancel" />
          </Button>,
          <Button key="full" onClick={() => syncSiteData(true)}>
            <FormattedMessage id="plugin.multilang.sync.all" />
          </Button>,
          <Button
            key="incremental"
            type="primary"
            onClick={() => syncSiteData(false)}
          >
            <FormattedMessage id="plugin.multilang.sync.addon" />
          </Button>,
        ]}
      >
        <FormattedMessage id="plugin.multilang.sync.content" />
      </Modal>
      {textLogVisible && (
        <TranslateTextLog
          open={textLogVisible}
          onCancel={() => setTextLogVisible(false)}
        />
      )}
      <VipModal />
    </PageContainer>
  );
};

export default PluginMultiLang;
