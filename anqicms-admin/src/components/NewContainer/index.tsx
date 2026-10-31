import { pluginGetMultiLangSites } from '@/services';
import { getSessionStore, setSessionStore } from '@/utils/store';
import { PageContainer } from '@ant-design/pro-components';
import { useEffect, useState } from 'react';

export type NewContainerProps = {
  title?: any;
  extra?: React.ReactNode;
  onTabChange?: (key: string, isSubsite: boolean) => void;
  children: React.ReactNode;
};

const NewContainer: React.FC<NewContainerProps> = (props) => {
  const [websites, setWebsites] = useState<any>(undefined);
  const [activeKey, setActiveKey] = useState<string>('');

  const getMultiLangSites = () => {
    pluginGetMultiLangSites({ type: 'multi' }).then((res) => {
      let data = (res.data || []).map((item: any) => {
        return {
          key: item.id,
          tab: item.name,
          is_main: item.is_main,
        };
      });
      setWebsites(data);
      let siteId = getSessionStore('sub-site-id');
      if (!siteId && data.length > 0) {
        siteId = data[0].key + '';
      }
      const site = data.find((item: any) => item.key === Number(siteId));

      let isSubsite = false;
      if (site) {
        isSubsite = true;
        if (site.is_main) {
          isSubsite = false;
        }
      }

      setSessionStore('is-sub-site', isSubsite);

      setActiveKey(siteId);
    });
  };

  useEffect(() => {
    getMultiLangSites();
  }, []);

  const handleChangeSite = (key: any) => {
    setActiveKey(key);
    setSessionStore('sub-site-id', key);
    const site = websites.find((item: any) => item.key === Number(key));
    let isSubsite = true;
    if (site.is_main) {
      isSubsite = false;
    }
    setSessionStore('is-sub-site', isSubsite);

    props.onTabChange?.(key, isSubsite);
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
