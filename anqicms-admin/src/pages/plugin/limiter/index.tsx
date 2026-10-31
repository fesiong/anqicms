import NewContainer from '@/components/NewContainer';
import {
  pluginGetBlockedIPs,
  pluginGetLimiterSetting,
  pluginRemoveBlockedIP,
  pluginSaveLimiterSetting,
} from '@/services';
import {
  ProForm,
  ProFormDigit,
  ProFormGroup,
  ProFormInstance,
  ProFormRadio,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Empty, Popover, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';

const PluginLimiter: React.FC<any> = () => {
  const formRef = React.createRef<ProFormInstance>();
  const [limiterSetting, setLimiterSetting] = useState<any>({});
  const [blockedIPs, setBlockedIPs] = useState<any[]>([]);
  const [memLimit, setMemLimit] = useState<boolean>(false);
  const [fetched, setFetched] = useState<boolean>(false);
  const [activeTabKey, setActiveTabKey] = useState<string>('setting');
  // 已访问过的 tab 常驻挂载（仅切换显示），避免切换 tab 时表单被卸载丢失未保存输入
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['setting']);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetLimiterSetting();
    let setting = res.data || {};
    // 需要转换数组成字符串
    setting.white_ips = setting.white_ips?.join('\n') || '';
    setting.black_ips = setting.black_ips?.join('\n') || '';
    setting.block_agents = setting.block_agents?.join('\n') || '';
    setting.allow_prefixes = setting.allow_prefixes?.join('\n') || '';

    setMemLimit(setting.mem_limit || false);
    setLimiterSetting(setting);
    setFetched(true);
  };

  const getBlockedIPs = () => {
    pluginGetBlockedIPs().then((res) => {
      setBlockedIPs(res.data || []);
    });
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      getBlockedIPs();
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
    getBlockedIPs();
  }, []);

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    // 提交前，需要将数据转换成数组
    let setting = values;
    // 需要转换数组成字符串
    setting.white_ips = (setting.white_ips.split('\n') || []).filter(
      (item: string) => item.length > 0,
    );
    setting.black_ips = (setting.black_ips.split('\n') || []).filter(
      (item: string) => item.length > 0,
    );
    setting.block_agents = (setting.block_agents.split('\n') || []).filter(
      (item: string) => item.length > 0,
    );
    setting.allow_prefixes = (setting.allow_prefixes.split('\n') || []).filter(
      (item: string) => item.length > 0,
    );

    pluginSaveLimiterSetting(setting)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const removeBlockedIP = (ip: string) => {
    pluginRemoveBlockedIP({ ip: ip }).then((res) => {
      message.success(res.msg);
      getBlockedIPs();
    });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
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
            label: intl.formatMessage({ id: 'plugin.limiter.tab.setting' }),
          },
          {
            key: 'blocked_ips',
            label: intl.formatMessage({
              id: 'plugin.limiter.blocked_ips',
            }),
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
              label={intl.formatMessage({ id: 'plugin.limiter.open.name' })}
              options={[
                {
                  label: intl.formatMessage({
                    id: 'plugin.limiter.open.false',
                  }),
                  value: false,
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.limiter.open.true',
                  }),
                  value: true,
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.limiter.description',
              })}
            />
            <ProFormRadio.Group
              name="is_allow_spider"
              label={intl.formatMessage({
                id: 'plugin.limiter.is_allow_spider',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.yes',
                  }),
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.limiter.is_allow_spider.description',
              })}
            />
            <ProFormRadio.Group
              name="ban_empty_agent"
              label={intl.formatMessage({
                id: 'plugin.limiter.ban_empty_agent',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.yes',
                  }),
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.limiter.ban_empty_agent.description',
              })}
            />
            <ProFormRadio.Group
              name="ban_empty_refer"
              label={intl.formatMessage({
                id: 'plugin.limiter.ban_empty_refer',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.yes',
                  }),
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.limiter.ban_empty_refer.description',
              })}
            />
            <ProFormRadio.Group
              name="mem_limit"
              label={intl.formatMessage({
                id: 'plugin.limiter.mem-limit',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.is_allow_spider.yes',
                  }),
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.limiter.mem-limit.description',
              })}
              fieldProps={{
                onChange: (e) => {
                  setMemLimit(e.target.value);
                },
              }}
            />
            {memLimit && (
              <ProFormDigit
                name={'mem_percent'}
                label={intl.formatMessage({
                  id: 'plugin.limiter.mem-percent',
                })}
                fieldProps={{
                  suffix: '%',
                }}
                extra={intl.formatMessage({
                  id: 'plugin.limiter.mem-percent.description',
                })}
              />
            )}
            <ProFormGroup>
              <ProFormDigit
                name={'max_requests'}
                label={intl.formatMessage({
                  id: 'plugin.limiter.max_requests',
                })}
                fieldProps={{
                  prefix: intl.formatMessage({
                    id: 'plugin.limiter.max_requests.prefix',
                  }),
                  suffix: intl.formatMessage({
                    id: 'plugin.limiter.max_requests.suffix',
                  }),
                }}
                extra={intl.formatMessage({
                  id: 'plugin.limiter.max_requests.description',
                })}
              />
              <ProFormDigit
                name={'block_hours'}
                label={intl.formatMessage({
                  id: 'plugin.limiter.block_hours',
                })}
                fieldProps={{
                  prefix: intl.formatMessage({
                    id: 'plugin.limiter.block_hours.prefix',
                  }),
                  suffix: intl.formatMessage({
                    id: 'plugin.limiter.block_hours.suffix',
                  }),
                }}
                extra={intl.formatMessage({
                  id: 'plugin.limiter.block_hours.description',
                })}
              />
            </ProFormGroup>
            <ProFormTextArea
              name={'white_ips'}
              label={intl.formatMessage({
                id: 'plugin.limiter.white_ips',
              })}
              extra={intl.formatMessage({
                id: 'plugin.limiter.white_ips.description',
              })}
            />
            <ProFormTextArea
              name={'black_ips'}
              label={intl.formatMessage({
                id: 'plugin.limiter.black_ips',
              })}
              extra={intl.formatMessage({
                id: 'plugin.limiter.black_ips.description',
              })}
            />
            <ProFormTextArea
              name={'block_agents'}
              label={intl.formatMessage({
                id: 'plugin.limiter.block_agents',
              })}
              extra={intl.formatMessage({
                id: 'plugin.limiter.block_agents.description',
              })}
            />
            <ProFormTextArea
              name={'allow_prefixes'}
              label={intl.formatMessage({
                id: 'plugin.limiter.allow_prefixes',
              })}
              placeholder={intl.formatMessage({
                id: 'plugin.limiter.allow_prefixes.placeholder',
              })}
              extra={intl.formatMessage({
                id: 'plugin.limiter.allow_prefixes.description',
              })}
            />
            <ProFormRadio.Group
              name="mem_limit"
              label={intl.formatMessage({
                id: 'plugin.limiter.mem_limit',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.mem_limit.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.limiter.mem_limit.yes',
                  }),
                },
              ]}
              fieldProps={{
                onChange: (e) => {
                  setMemLimit(e.target.value);
                },
              }}
              extra={intl.formatMessage({
                id: 'plugin.limiter.mem_limit.description',
              })}
            />
            {memLimit && (
              <ProFormDigit
                name={'mem_percent'}
                label={intl.formatMessage({
                  id: 'plugin.limiter.mem_percent',
                })}
                fieldProps={{
                  suffix: '%',
                }}
                placeholder={intl.formatMessage({
                  id: 'plugin.limiter.mem_percent.placeholder',
                })}
                extra={intl.formatMessage({
                  id: 'plugin.limiter.mem_percent.description',
                })}
              />
            )}
          </ProForm>
          </div>
        )}
        {fetched && visitedTabs.includes('blocked_ips') && (
          <div style={{ display: activeTabKey === 'blocked_ips' ? 'block' : 'none' }}>
          <div>
            {blockedIPs.length > 0 ? (
              <Space wrap size={[16, 16]}>
                {blockedIPs.map((item, index) => (
                  <Popover
                    key={index}
                    title={intl.formatMessage({
                      id: 'plugin.limiter.blocked_ips.remove',
                    })}
                    content={
                      <Space align="center" size={16}>
                        <Button
                          type="link"
                          onClick={() => removeBlockedIP(item.ip)}
                        >
                          <FormattedMessage id="plugin.limiter.blocked_ips.remove.yes" />
                        </Button>
                      </Space>
                    }
                  >
                    <Tag key={index}>
                      <span>{item.ip}</span>
                      <span>
                        <FormattedMessage id="plugin.limiter.blocked_ips.ended" />
                        {dayjs(item.block_time * 1000).format('MM-DD HH:mm')}
                      </span>
                    </Tag>
                  </Popover>
                ))}
              </Space>
            ) : (
              <Empty />
            )}
          </div>
          </div>
        )}
      </Card>
    </NewContainer>
  );
};

export default PluginLimiter;
