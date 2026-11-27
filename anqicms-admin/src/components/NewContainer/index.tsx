import { pluginGetMultiLangSites } from '@/services';
import { getSessionStore, setSessionStore } from '@/utils/store';
import { PageContainer } from '@ant-design/pro-components';
import { useEffect, useState } from 'react';
import { normalizeSites, resolveSiteSelection } from './site';

export type NewContainerProps = {
  title?: any;
  extra?: React.ReactNode;
  onTabChange?: (key: string, isSubsite: boolean) => void;
  children: React.ReactNode;
};

const NewContainer: React.FC<NewContainerProps> = (props) => {
  const [websites, setWebsites] = useState<any[]>([]);
  const [activeKey, setActiveKey] = useState<string>('');

  const getMultiLangSites = () => {
    pluginGetMultiLangSites({ type: 'multi' }).then((res) => {
      const data = normalizeSites(res.data || []);
      setWebsites(data);
      const storedSiteId = getSessionStore('sub-site-id');
      const selection = resolveSiteSelection(data, storedSiteId);
      setActiveKey(selection.activeKey);
      setSessionStore('is-sub-site', selection.isSubsite);
      if (data.length > 0 && String(storedSiteId ?? '') !== selection.activeKey) {
        setSessionStore('sub-site-id', selection.activeKey);
        props.onTabChange?.(selection.activeKey, selection.isSubsite);
      }
    });
  };

  useEffect(() => {
    getMultiLangSites();
  }, []);

  const handleChangeSite = (key: any) => {
    const site = websites.find((item) => item.key === Number(key));
    if (!site) return;
    const activeKey = String(site.key);
    const isSubsite = !site.is_main;
    setActiveKey(activeKey);
    setSessionStore('sub-site-id', activeKey);
    setSessionStore('is-sub-site', isSubsite);
    props.onTabChange?.(activeKey, isSubsite);
  };

  return (
    <PageContainer
      tabList={websites}
      tabActiveKey={activeKey}
      onTabChange={handleChangeSite}
      title={props.title}
      extra={props.extra}
    >
      {props.children}
    </PageContainer>
  );
};

export default NewContainer;
