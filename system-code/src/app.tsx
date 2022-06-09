import type { Settings as LayoutSettings } from '@ant-design/pro-layout';
import { SettingDrawer } from '@ant-design/pro-layout';
import { PageLoading } from '@ant-design/pro-layout';
import type { RunTimeLayoutConfig } from 'umi';
import { history, Link } from 'umi';
import RightContent from '@/components/RightContent';
import Footer from '@/components/Footer';
import { getAdminInfo } from './services/admin';
import { BookOutlined, LinkOutlined } from '@ant-design/icons';
import defaultSettings from '../config/defaultSettings';
import HeaderContent from './components/headerContent';
import { getSettingSystem } from './services/setting';

const loginPath = '/login';

/** 获取用户信息比较慢的时候会展示一个 loading */
export const initialStateConfig = {
  loading: <PageLoading />,
};

/**
 * @see  https://umijs.org/zh-CN/plugins/plugin-initial-state
 * */
export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  currentUser?: any;
  system?: any;
  loading?: boolean;
  fetchUserInfo?: () => Promise<any | undefined>;
  fetchSystemSetting?: () => Promise<any | undefined>;
}> {
  const fetchUserInfo = async () => {
    try {
      const msg = await getAdminInfo();
      return msg.data;
    } catch (error) {
      history.push(loginPath);
    }
    return undefined;
  };
  const fetchSystemSetting = async () => {
    try {
      const msg = await getSettingSystem();
      return msg.data?.system;
    } catch (error) {
      //
    }
    return undefined;
  };
  // 如果是登录页面，不执行
  if (history.location.pathname !== loginPath) {
    const currentUser = await fetchUserInfo();
    const system = await fetchSystemSetting();

    return {
      fetchSystemSetting,
      fetchUserInfo,
      currentUser,
      settings: defaultSettings,
      system: system,
    };
  }
  return {
    fetchSystemSetting,
    fetchUserInfo,
    settings: defaultSettings,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({ initialState, setInitialState }) => {
  return {
    rightContentRender: () => <RightContent />,
    disableContentMargin: false,
    footerRender: () => <Footer />,
    onPageChange: () => {
      const { location } = history;
      // 如果没有登录，重定向到 login
      if (!initialState?.currentUser && location.pathname !== loginPath) {
        history.push(loginPath);
      }
    },
    links: [
      <a href="https://www.kandaoni.com/anqicms" target="_blank">
        <LinkOutlined />
        <span>安企CMS(AnqiCMS)</span>
      </a>,
    ],
    headerContentRender: () => <HeaderContent />,
    menuHeaderRender: undefined,
    // 自定义 403 页面
    // unAccessible: <div>unAccessible</div>,
    // 增加一个 loading 的状态
    childrenRender: (children, props) => {
      // if (initialState?.loading) return <PageLoading />;
      return (
        <>
          {children}
          {/* {!props.location?.pathname?.includes('/login') && (
            <SettingDrawer
              enableDarkTheme
              settings={initialState?.settings}
              onSettingChange={(settings) => {
                setInitialState((preInitialState) => ({
                  ...preInitialState,
                  settings,
                }));
              }}
            />
          )} */}
        </>
      );
    },
    ...initialState?.settings,
  };
};
