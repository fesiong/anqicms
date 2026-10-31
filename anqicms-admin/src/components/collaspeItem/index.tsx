import { RightOutlined } from '@ant-design/icons';
import classNames from 'classnames';
import React, { useEffect, useState } from 'react';
import './index.less';

export type CollapseItemProps = {
  header: React.ReactNode;
  open?: boolean;
  className?: string;
  style?: React.CSSProperties;
  showArrow?: boolean;
  extra?: React.ReactNode;
  children?: React.ReactNode;
};

const CollapseItem: React.FC<CollapseItemProps> = (props) => {
  const [open, setOpen] = useState<boolean>(false);

  useEffect(() => {
    setOpen(props.open || false);
  }, []);

  const onChangeOpen = () => {
    setOpen(!open);
  };

  const itemCls = classNames(
    {
      [`collaspe-item`]: true,
      [`collaspe-item-active`]: open,
    },
    props.className,
  );

  return (
    <div className={itemCls} style={props.style}>
      <div className="collaspe-header" onClick={onChangeOpen}>
        {props.showArrow && <RightOutlined className="collaspe-icon" />}
        <div className="collaspe-header-text">{props.header}</div>
        {props.extra && <div className={`collaspe-extra`}>{props.extra}</div>}
      </div>
      <div className="collaspe-content">{props.children}</div>
    </div>
  );
};

export default CollapseItem;
