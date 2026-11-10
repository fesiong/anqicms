import NewContainer from '@/components/NewContainer';
import { getCategories, getModules } from '@/services';
import {
  pluginBuildSitemap,
  pluginGetSitemap,
  pluginSaveSitemap,
} from '@/services/plugin/sitemap';
import {
  ProForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';

const PluginSitemap: React.FC<any> = () => {
  const [sitemapSetting, setSitemapSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const [activeTabKey, setActiveTabKey] = useState<string>('setting');
  // 已访问过的 tab 常驻挂载（仅切换显示），避免切换 tab 时表单被卸载丢失未保存输入
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['setting']);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetSitemap();
    let setting = res.data || {};
    setSitemapSetting(setting);
    getCategories().then((res) => {
      setCategories(res.data || []);
    });
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveSitemap(values)
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

  // 生成只按「已保存的配置」重建，不带任何表单值：
  // 早先后端会拿请求体当全量配置覆盖，表单取不到值时点一下就清空真实配置。
  const rebuildSitemap = () => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginBuildSitemap()
      .then((res) => {
        message.info(res.msg);
        if (res.code === 0) {
          setSitemapSetting(res.data);
        }
      })
      .finally(() => {
        hide();
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
            label: intl.formatMessage({ id: 'plugin.sitemap.tab.setting' }),
          },
          {
            key: 'build',
            label: intl.formatMessage({ id: 'plugin.sitemap.action' }),
          },
        ]}
      >
        <Alert
          message={
            <div>
              <div>
                <FormattedMessage id="plugin.sitemap.tips1" />
              </div>
              <div>
                <FormattedMessage id="plugin.sitemap.tips2" />
              </div>
            </div>
          }
        />
        {fetched && visitedTabs.includes('setting') && (
          <div style={{ display: activeTabKey === 'setting' ? 'block' : 'none' }} className="mt-normal">
            <ProForm
              onFinish={onSubmit}
              initialValues={sitemapSetting}
            >
              <ProFormRadio.Group
                name="type"
                label={intl.formatMessage({ id: 'plugin.sitemap.type' })}
                options={[
                  { value: 'txt', label: 'txt' },
                  { value: 'xml', label: 'xml' },
                ]}
              />
              <ProFormRadio.Group
                name="auto_build"
                label={intl.formatMessage({
                  id: 'plugin.sitemap.auto-build',
                })}
                options={[
                  {
                    value: 0,
                    label: intl.formatMessage({
                      id: 'plugin.sitemap.auto-build.manual',
                    }),
                  },
                  {
                    value: 1,
                    label: intl.formatMessage({
                      id: 'plugin.sitemap.auto-build.auto',
                    }),
                  },
                ]}
              />
              <ProFormRadio.Group
                name="exclude_tag"
                label={intl.formatMessage({
                  id: 'plugin.sitemap.exclude-tag',
                })}
                options={[
                  {
                    value: false,
                    label: intl.formatMessage({
                      id: 'plugin.sitemap.exclude-tag.no',
                    }),
                  },
                  {
                    value: true,
                    label: intl.formatMessage({
                      id: 'plugin.sitemap.exclude-tag.yes',
                    }),
                  },
                ]}
              />
              <ProFormDigit
                name="page_size"
                label={intl.formatMessage({
                  id: 'plugin.sitemap.page-size',
                })}
                extra={intl.formatMessage({
                  id: 'plugin.sitemap.page-size.description',
                })}
              />
              <ProFormSelect
                name={'exclude_module_ids'}
                label={intl.formatMessage({
                  id: 'plugin.sitemap.exculde-module',
                })}
                mode="multiple"
                request={async () => {
                  let res = await getModules({});
                  const tmpModules = (res.data || []).map((item: any) => ({
                    label: item.name,
                    value: item.id,
                  }));
                  return tmpModules;
                }}
                placeholder={intl.formatMessage({
                  id: 'plugin.sitemap.exculde-module.description',
                })}
              />
              <ProFormSelect
                name={'exclude_category_ids'}
                label={intl.formatMessage({
                  id: 'plugin.sitemap.exculde-category',
                })}
                mode="multiple"
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
                }}
                placeholder={intl.formatMessage({
                  id: 'plugin.sitemap.exculde-category.description',
                })}
              />
              <ProFormSelect
                name={'exclude_page_ids'}
                label={intl.formatMessage({
                  id: 'plugin.sitemap.exculde-page',
                })}
                mode="multiple"
                request={async () => {
                  let res = await getCategories({ type: 3 });
                  const tmpData = (res.data || []).map((item: any) => ({
                    label: item.title,
                    value: item.id,
                  }));
                  return tmpData;
                }}
                placeholder={intl.formatMessage({
                  id: 'plugin.sitemap.exculde-page.description',
                })}
              />
            </ProForm>
          </div>
        )}
        {fetched && visitedTabs.includes('build') && (
          <div style={{ display: activeTabKey === 'build' ? 'block' : 'none' }} className="mt-normal">
            <div>
              <FormattedMessage id="plugin.sitemap.action.tips" />
            </div>
            <ProFormText
              readonly
              label={intl.formatMessage({ id: 'plugin.sitemap.last-time' })}
              fieldProps={{
                value: dayjs(sitemapSetting.updated_time * 1000).format(
                  'YYYY-MM-DD HH:mm',
                ),
              }}
            />
            <Space size={20}>
              <Button
                type="primary"
                onClick={() => {
                  rebuildSitemap();
                }}
              >
                <FormattedMessage id="plugin.sitemap.build" />
              </Button>
              <Button
                onClick={() => {
                  window.open(sitemapSetting.sitemap_url);
                }}
              >
                <FormattedMessage id="plugin.sitemap.view" />
              </Button>
            </Space>
          </div>
        )}
      </Card>
    </NewContainer>
  );
};

export default PluginSitemap;
