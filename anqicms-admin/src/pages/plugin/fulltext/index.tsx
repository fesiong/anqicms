import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  getModules,
  pluginGetFulltextConfig,
  pluginGetFulltextStatus,
  pluginRebuildFulltextIndex,
  pluginSaveFulltextConfig,
} from '@/services';
import {
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Modal, Progress, Space, message } from 'antd';
import React, { useEffect, useState } from 'react';

let running = false;
let intXhr: any = null;

const PluginFulltext: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const [modules, setModules] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [task, setTask] = useState<any>(null);
  const [engine, setEngine] = useState<string>('');
  const intl = useIntl();

  const syncTask = async () => {
    pluginGetFulltextStatus().then((res) => {
      if (res.data) {
        setTask(res.data);
        if (res.data.status === 1) {
          running = true;
        } else {
          running = false;
          clearInterval(intXhr);
        }
      } else {
        if (running) {
          running = false;
          clearInterval(intXhr);
        }
        setTask(null);
      }
    });
  };

  const getSetting = async () => {
    const res = await getModules();
    const data = res.data || [];
    const tmpData = [];
    // eslint-disable-next-line guard-for-in
    for (let i in data) {
      tmpData.push({ label: data[i].name, value: data[i].id });
    }
    setModules(tmpData);
  };

  const onTabChange = (key: string) => {
    running = false;
    clearInterval(intXhr);
    getSetting().then(() => {
      setNewKey(key);
    });
    // 定时查询task
    intXhr = setInterval(() => {
      syncTask();
    }, 1000);
  };

  useEffect(() => {
    getSetting();
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
    pluginSaveFulltextConfig(values)
      .then((res) => {
        message.success(res.msg);
        running = false;
        clearInterval(intXhr);
        // 定时查询task
        intXhr = setInterval(() => {
          syncTask();
        }, 1000);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const handleRebuild = () => {
    if (running) {
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.fulltext.rebuild.confirm' }),
      onOk: () => {
        running = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        pluginRebuildFulltextIndex({})
          .then((res) => {
            message.info(res.msg);
            // 马上执行一遍
            intXhr = setInterval(() => {
              syncTask();
            }, 1000);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <Alert
          style={{ marginBottom: 20 }}
          description={
            <div>
              <p>
                <FormattedMessage id="plugin.fulltext.tips" />
              </p>
              {task !== null && task.status === -1 && (
                <p style={{ color: '#f30' }}>{task.msg}</p>
              )}
              {isOpen && (
                <Button key="clean" onClick={() => handleRebuild()}>
                  <FormattedMessage id="plugin.fulltext.rebuild" />
                </Button>
              )}
            </div>
          }
        />
        <ProForm
          request={async () => {
            const res = await pluginGetFulltextConfig();
            const data = res.data || {};
            setEngine(data.engine || 'default');
            setIsOpen(data.open || false);
            return data;
          }}
          onFinish={onSubmit}
          title={intl.formatMessage({ id: 'menu.plugin.system.fulltext' })}
        >
          <ProFormRadio.Group
            name={'open'}
            label={intl.formatMessage({ id: 'plugin.fulltext.open.name' })}
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.fulltext.open.false' }),
                value: false,
              },
              {
                label: intl.formatMessage({ id: 'plugin.fulltext.open.true' }),
                value: true,
              },
            ]}
            fieldProps={{
              onChange: (e) => {
                setIsOpen(e.target.value);
              },
            }}
          />
          {isOpen && (
            <>
              <ProFormRadio.Group
                name={'engine'}
                label={intl.formatMessage({
                  id: 'plugin.fulltext.engine',
                })}
                options={[
                  {
                    label: intl.formatMessage({
                      id: 'plugin.fulltext.wukong',
                    }),
                    value: 'default',
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.fulltext.elasticsearch',
                    }),
                    value: 'elasticsearch',
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.fulltext.zincSearch',
                    }),
                    value: 'zincsearch',
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.fulltext.meilisearch',
                    }),
                    value: 'meilisearch',
                  },
                ]}
                fieldProps={{
                  onChange: (e) => {
                    setEngine(e.target.value);
                  },
                }}
                disabled={isVip === false}
                extra={
                  !isVip ? (
                    <div
                      className="link"
                      onClick={() => {
                        checkVip(() => {});
                      }}
                    >
                      <FormattedMessage id="plugin.fulltext.vip-tip" />
                    </div>
                  ) : null
                }
              />
              {engine !== 'default' && (
                <>
                  <ProFormText
                    name="engine_url"
                    label={intl.formatMessage({
                      id: 'plugin.fulltext.engine-url',
                    })}
                    width="lg"
                  />
                  {engine !== 'meilisearch' && (
                    <ProFormText
                      name="engine_user"
                      label={intl.formatMessage({
                        id: 'plugin.fulltext.engine-user',
                      })}
                      width="lg"
                    />
                  )}
                  <ProFormText
                    name="engine_pass"
                    label={intl.formatMessage({
                      id: 'plugin.fulltext.engine-password',
                    })}
                    width="lg"
                  />
                </>
              )}
              <ProFormDigit
                name="contain_length"
                label={intl.formatMessage({
                  id: 'plugin.fulltext.contain_length',
                })}
                width="lg"
                extra={
                  <FormattedMessage id="plugin.fulltext.contain_length.tips" />
                }
              />
              <ProFormDigit
                name="ranking_score"
                label={intl.formatMessage({
                  id: 'plugin.fulltext.ranking_score',
                })}
                width="lg"
                extra={
                  <FormattedMessage id="plugin.fulltext.ranking_score.tips" />
                }
              />
              <ProFormRadio.Group
                name={'use_content'}
                label={intl.formatMessage({
                  id: 'plugin.fulltext.use_content.name',
                })}
                options={[
                  {
                    label: intl.formatMessage({
                      id: 'plugin.fulltext.use_content.false',
                    }),
                    value: false,
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.fulltext.use_content.true',
                    }),
                    value: true,
                  },
                ]}
              />
              <ProFormCheckbox.Group
                name={'modules'}
                label={intl.formatMessage({
                  id: 'plugin.fulltext.modules.name',
                })}
                options={modules}
              />
              <ProFormCheckbox.Group
                label={intl.formatMessage({
                  id: 'plugin.fulltext.search.name',
                })}
              >
                <Space>
                  <ProFormCheckbox
                    name={'use_archive'}
                    disabled
                    initialValue={true}
                  >
                    <FormattedMessage id="plugin.fulltext.search.archive" />
                  </ProFormCheckbox>
                  <ProFormCheckbox name={'use_category'}>
                    <FormattedMessage id="plugin.fulltext.search.category" />
                  </ProFormCheckbox>
                  <ProFormCheckbox name={'use_tag'}>
                    <FormattedMessage id="plugin.fulltext.search.tag" />
                  </ProFormCheckbox>
                </Space>
              </ProFormCheckbox.Group>
            </>
          )}
        </ProForm>
      </Card>
      {task !== null && task.status === 1 && (
        <Modal
          title={intl.formatMessage({ id: 'plugin.fulltext.indexing' })}
          open={true}
          footer={null}
        >
          <div className="task-progress">
            <Progress
              percent={
                task.total > 0
                  ? Number(((task.current / task.total) * 100).toFixed())
                  : 1
              }
            />
          </div>
          <div className="task-message">{task.msg}</div>
        </Modal>
      )}
      <VipModal />
    </NewContainer>
  );
};

export default PluginFulltext;
