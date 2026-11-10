import { Dropdown, MenuProps } from 'antd';
import type { DropDownProps } from 'antd/es/dropdown';
import classNames from 'classnames';
import type { MenuProps as RcMenuProps } from 'rc-menu';
import React from 'react';
import styles from './index.less';

export type HeaderDropdownProps = {
  overlayClassName?: string;
  placement?:
    | 'bottomLeft'
    | 'bottomRight'
    | 'topLeft'
    | 'topCenter'
    | 'topRight'
    | 'bottomCenter';
  menu?: MenuProps & {
    activeKey?: RcMenuProps['activeKey'];
  };
} & Omit<DropDownProps, 'overlay'>;

const HeaderDropdown: React.FC<HeaderDropdownProps> = ({
  overlayClassName: cls,
  ...restProps
}) => (
  <Dropdown
    overlayClassName={classNames(styles.container, cls)}
    {...restProps}
  />
);

export default HeaderDropdown;
