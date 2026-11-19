import { Image, Tag } from 'antd';
import React from 'react';
import { normalizeTextsItemType, normalizeTextsItems, textsItemDisplayText } from './types';

export type CustomFieldTextsViewProps = {
  /** 任意历史形态的 texts 值 */
  value?: any;
  /** 为空时不渲染空行 */
  showEmptyKey?: boolean;
};

/**
 * texts 字段的只读展示：按条目类型决定展示方式。
 * 多值条目（text/textarea/checkbox）拼成一段文本，image/images 出图，file 出链接。
 */
const CustomFieldTextsView: React.FC<CustomFieldTextsViewProps> = (props) => {
  const items = normalizeTextsItems(props.value);
  if (items.length === 0) {
    return null;
  }
  return (
    <div className="text-groups">
      {items.map((item, index) => {
        const type = normalizeTextsItemType(item.type);
        const images = (item.values || []).filter((v) => v !== '');
        return (
          <div className="text-group" key={index}>
            <div className="text-key">{item.key}:</div>
            <div className="text-value">
              {type === 'image' ? (
                item.value ? (
                  <Image height={80} src={item.value} />
                ) : null
              ) : type === 'images' ? (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {images.map((src, imageIdx) => (
                    <Image key={`${src}-${imageIdx}`} height={80} src={src} />
                  ))}
                </div>
              ) : type === 'file' ? (
                item.value ? (
                  <a href={item.value} target="_blank" rel="noreferrer">
                    {item.value}
                  </a>
                ) : null
              ) : type === 'checkbox' ? (
                (item.values || [])
                  .filter((v) => v !== '')
                  .map((value) => <Tag key={value}>{value}</Tag>)
              ) : type === 'color' ? (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                  <span
                    style={{
                      display: 'inline-block',
                      width: 16,
                      height: 16,
                      borderRadius: 3,
                      border: '1px solid #ddd',
                      background: item.value,
                    }}
                  />
                  {item.value}
                </span>
              ) : (
                <span style={{ whiteSpace: 'pre-wrap' }}>{textsItemDisplayText(item)}</span>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default CustomFieldTextsView;
