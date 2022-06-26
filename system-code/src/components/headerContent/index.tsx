import { Menu, MenuProps, Space } from 'antd';
import React from 'react';
import { history, useModel } from 'umi';
import './index.less';
import routes from '../../../config/routes';

const GlobalHeaderContent: React.FC = (props) => {
  const { initialState } = useModel('@@initialState');

  const getSelectKey = () => {
    const selectPath = history.location.pathname;
    for (let i in routes) {
      if (routes[i].path && selectPath.indexOf(routes[i].path) === 0) {
        return i
      }
    }

    return '';
  }

  const selectKey: string = getSelectKey();

  if (!initialState || !initialState.settings) {
    return null;
  }

  const onClickMenu = (e: any) => {
    let index = e.key;
    let current: any = routes[index] || null;
    if (current != null) {
      // preview单独处理
      if (current.path == "/preview") {
        window.open(initialState.system?.base_url || '')
        return;
      }

      if (current.routes) {
        history.push(current.routes[0].path)
      } else {
        history.push(current.path)
      }
    }

  }

  return (
    <div className='header-nav'>
      <Menu onClick={onClickMenu} selectedKeys={[selectKey]} theme={'dark'} mode="horizontal">
        {routes.map((item, index) => {
          if(!item.hideInTop && item.name) {
            return <Menu.Item key={index}>{item.name}</Menu.Item>
          }
          return null
        })}
      </Menu>
    </div>
  );
};
export default GlobalHeaderContent;
