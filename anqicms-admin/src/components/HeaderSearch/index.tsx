import { SearchOutlined } from '@ant-design/icons';
import { history, useIntl } from '@umijs/max';
import { AutoComplete, Input, InputRef } from 'antd';
import type { AutoCompleteProps } from 'antd/es/auto-complete';
import useMergedState from 'rc-util/es/hooks/useMergedState';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import routes from '../../../config/routes';
import {
  collectSearchItems,
  matchSearchItems,
  type MenuRoute,
  type SearchItem,
} from './menuSearch';

import classNames from 'classnames';
import styles from './index.less';

export type HeaderSearchProps = {
  onSearch?: (value?: string) => void;
  onChange?: (value?: string) => void;
  onOpenChange?: (b: boolean) => void;
  className?: string;
  placeholder?: string;
  options?: AutoCompleteProps['options'];
  defaultOpen?: boolean;
  open?: boolean;
  defaultValue?: string;
  value?: string;
};

/** 父级标题的弱化配色，让「分类 / 功能名」在主次上分得开 */
const parentStyle: React.CSSProperties = { color: 'rgba(0, 0, 0, 0.45)' };
const separatorStyle: React.CSSProperties = {
  color: 'rgba(0, 0, 0, 0.2)',
  margin: '0 4px',
};

const HeaderSearch: React.FC<HeaderSearchProps> = (props) => {
  const { className, defaultValue, onOpenChange, placeholder, defaultOpen } =
    props;
  const intl = useIntl();

  const inputRef = useRef<InputRef | null>(null);

  const [value, setValue] = useMergedState<string | undefined>(defaultValue, {
    value: props.value,
    onChange: props.onChange,
  });

  const [searchMode, setSearchMode] = useMergedState(defaultOpen ?? false, {
    value: props.open,
    onChange: onOpenChange,
  });

  const [options, setOptions] = useState<AutoCompleteProps['options']>([]);

  /**
   * 菜单标题按「完整层级」取 i18n（如 menu.plugin.seo.rewrite），
   * 取不到时逐级回退到「末两段 → 叶子名」，避免某个节点漏配 key 时
   * 整个搜索结果里出现裸的 menu.xxx 文案。
   */
  const resolveTitle = (names: string[]): string => {
    if (!names.length) return '';
    const candidates = ['menu.' + names.join('.')];
    if (names.length > 2) candidates.push('menu.' + names.slice(-2).join('.'));
    if (names.length > 1) candidates.push('menu.' + names[names.length - 1]);

    for (const id of candidates) {
      const text = intl.formatMessage({ id });
      if (text && text !== id) return text;
    }
    return names[names.length - 1];
  };

  // 菜单结构随语言切换而变化，重新生成索引。
  // 依赖用 intl.locale 而非 intl 对象本身：后者若是每次 render 的新引用，
  // 会让下面的 useEffect 反复 setOptions，形成渲染循环。
  const allItems = useMemo(
    () => collectSearchItems(routes as unknown as MenuRoute[], resolveTitle),
    [intl.locale],
  );

  const toOptions = (items: SearchItem[]): AutoCompleteProps['options'] =>
    items.map((item) => ({
      // value 用 path：同名功能（如「用户管理」在多处出现）不会互相覆盖
      value: item.path,
      label: item.parents.length ? (
        <>
          <span style={parentStyle}>{item.parents[item.parents.length - 1]}</span>
          <span style={separatorStyle}>/</span>
          <span>{item.title}</span>
        </>
      ) : (
        item.title
      ),
    }));

  const applyMatch = (keyword?: string) => {
    const items = matchSearchItems(allItems, keyword);
    setOptions(toOptions(items));
    return items;
  };

  useEffect(() => {
    applyMatch('');
    let width = window.innerWidth;
    if (width > 1280) {
      setSearchMode(true);
    }
  }, [allItems]);

  const go = (item?: SearchItem) => {
    if (!item) return;
    history.push(item.path);
    setValue('');
    setSearchMode(false);
  };

  const onChangeValue = (val: string) => {
    setValue(val);
    applyMatch(val);
  };

  const inputClass = classNames(styles.input, {
    [styles.show]: searchMode,
  });

  return (
    <div
      className={classNames(className, styles.headerSearch)}
      onClick={() => {
        setSearchMode(true);
        if (searchMode && inputRef.current) {
          inputRef.current.focus();
        }
      }}
      onTransitionEnd={({ propertyName }) => {
        if (propertyName === 'width' && !searchMode) {
          if (onOpenChange) {
            onOpenChange(searchMode);
          }
        }
      }}
    >
      <AutoComplete
        key="AutoComplete"
        className={inputClass}
        value={value}
        options={options}
        // 已自行按标题/分类/路径过滤，交给 antd 再用 path 过滤会把结果全筛掉
        filterOption={false}
        onChange={onChangeValue}
        onSelect={(val: string) => {
          go(allItems.find((item) => item.path === val));
        }}
      >
        <Input
          ref={inputRef}
          defaultValue={defaultValue}
          aria-label={placeholder}
          placeholder={placeholder}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              // 回车直接进第一条，与下拉框顶部结果保持一致
              go(matchSearchItems(allItems, value)[0]);
            }
          }}
          onBlur={() => {
            setSearchMode(false);
          }}
        />
      </AutoComplete>
      <SearchOutlined
        className={styles.searchIcon}
        key="Icon"
        style={{
          cursor: 'pointer',
        }}
      />
    </div>
  );
};

export default HeaderSearch;
