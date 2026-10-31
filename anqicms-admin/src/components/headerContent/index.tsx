import { history, useIntl, useModel } from '@umijs/max';
import { Menu } from 'antd';
import React from 'react';
import routes from '../../../config/routes';
import {
  isShopMenuPath,
  isShopMode,
  SHOP_MODE_HIDE_IN_TOP,
} from '../../utils/shopMode';
import './index.less';

const GlobalHeaderContent: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const intl = useIntl();

  const getSelectKey = () => {
    let selectPath = history.location.pathname;
    if (selectPath.indexOf('/system') === 0) {
      // 截取/system 之后的字符
      selectPath = selectPath.substring(7);
    }
    for (let i in routes) {
      if (routes[i].path && selectPath.indexOf(routes[i].path) === 0) {
        return i;
      }
    }

    return '';
  };

  const selectKey: string = getSelectKey();

  if (!initialState || !initialState.settings) {
    return null;
  }

  const onClickMenu = async (current: any) => {
    // preview单独处理
    if (current.key === '/preview') {
      let baseUrl = '';
      if (!initialState.system) {
        const system = await initialState?.fetchSystemSetting?.();
        if (system) {
          await setInitialState((s) => ({
            ...s,
            system: system,
          }));
        }
        baseUrl = system?.base_url || '';
      } else {
        baseUrl = initialState.system?.base_url || '';
      }
      window.open(baseUrl);
      return;
    }
    history.push(current.key);
  };

  let permissions =
    initialState?.currentUser?.group?.setting?.permissions || [];
  const shopMode = isShopMode(initialState?.system);
  const headerMenu = routes.reduce((pre: any, cur: any) => {
    if (!cur.hideInTop && cur.name) {
      // 普通内容模式下顶部导航同样不显示商城分组（清单见 utils/shopMode.ts，
      // 与 app.tsx 的侧栏过滤共用一份，避免两处漏改导致顶栏残留商城菜单）
      if (!shopMode && isShopMenuPath(cur.path, true)) {
        return pre;
      }
      // 商城模式把「设置」「功能」收进侧栏，顶栏不显示；普通模式照主干放在顶栏
      if (shopMode && SHOP_MODE_HIDE_IN_TOP.indexOf(cur.path) !== -1) {
        return pre;
      }
      let path = cur.path;
      if (cur.routes) {
        path = cur.routes[0].path;
        for (let j in cur.routes) {
          if (permissions.indexOf(cur.routes[j].path) !== -1) {
            path = cur.routes[j].path;
            break;
          }
        }
      }

      if (permissions.length > 0) {
        for (let j in permissions) {
          if (permissions[j].indexOf(cur.path) === 0) {
            pre.push({
              key: path,
              label: intl.formatMessage({ id: 'menu.' + cur.name }),
            });
            break;
          }
        }
      } else {
        pre.push({
          key: path,
          label: intl.formatMessage({ id: 'menu.' + cur.name }),
        });
      }
    }
    return pre;
  }, []);

  return (
    <div className="header-nav">
      <Menu
        onClick={onClickMenu}
        // 顶部导航配色跟随站点模式：普通内容模式（shop_mode=0）用浅色菜单配白色顶栏，
        // 商城模式（1）用深色菜单配深色顶栏。写死 dark 会让普通版顶栏中间出现黑块。
        theme={shopMode ? 'dark' : 'light'}
        selectedKeys={[selectKey]}
        mode="horizontal"
        items={headerMenu}
      ></Menu>
    </div>
  );
};
export default GlobalHeaderContent;
