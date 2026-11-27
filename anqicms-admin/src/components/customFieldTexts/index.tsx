import AttachmentSelect from '@/components/attachment';
import ImageItem from '@/components/attachment/image';
import {
  Checkbox,
  ColorPicker,
  DatePicker,
  Input,
  InputNumber,
  Popconfirm,
  Radio,
  Select,
  Space,
  Tag,
  TimePicker,
} from 'antd';
import {
  DeleteOutlined,
  DownOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
  UpOutlined,
} from '@ant-design/icons';
import { ProFormText } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import React from 'react';
import {
  CUSTOM_FIELD_TEXTS_TYPES,
  changeTextsItemType,
  createTextsItem,
  normalizeTextsItemType,
  normalizeTextsItems,
  parseTextsOptions,
  serializeTextsItems,
} from './types';
import type { CustomFieldTextsItem, CustomFieldTextsItemType } from './types';

export type CustomFieldTextsEditorProps = {
  /** 任意历史形态的 texts 值：数组 / JSON 字符串 / CustomField 包装体 */
  value?: any;
  /** 回写序列化后的条目数组（text 类型不带 type 字段，与历史数据同形） */
  onChange?: (items: any[]) => void;
  label?: React.ReactNode;
  /** 原始 CustomField，用来取选项（content）等信息 */
  field?: any;
  disabled?: boolean;
};

/** 时间：自己按正则拆，避免依赖 dayjs 的 customParseFormat 插件 */
const parseTimeValue = (value: string) => {
  const matched = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/.exec((value || '').trim());
  if (!matched) {
    return undefined;
  }
  return dayjs()
    .hour(Number(matched[1]))
    .minute(Number(matched[2]))
    .second(Number(matched[3] || 0));
};

/** 日期/日期时间：把 "YYYY-MM-DD HH:mm:ss" 归一成 ISO 形态再交给 dayjs */
const parseDateValue = (value: string) => {
  const text = (value || '').trim();
  if (!text) {
    return undefined;
  }
  const parsed = dayjs(text.replace(' ', 'T'));
  return parsed.isValid() ? parsed : undefined;
};

const CustomFieldTextsEditor: React.FC<CustomFieldTextsEditorProps> = (props) => {
  const { value, onChange, label, field, disabled } = props;
  const intl = useIntl();

  const items = normalizeTextsItems(value);
  const options = parseTextsOptions(field?.content);

  const emit = (next: CustomFieldTextsItem[]) => {
    onChange?.(serializeTextsItems(next));
  };

  const updateItem = (index: number, patch: Partial<CustomFieldTextsItem>) => {
    const next = items.map((item, i) => (i === index ? { ...item, ...patch } : item));
    emit(next);
  };

  const moveItem = (index: number, offset: number) => {
    const target = index + offset;
    if (target < 0 || target >= items.length) {
      return;
    }
    const next = [...items];
    const tmp = next[index];
    next[index] = next[target];
    next[target] = tmp;
    emit(next);
  };

  const removeItem = (index: number) => {
    emit(items.filter((_, i) => i !== index));
  };

  const addItem = () => {
    emit([...items, createTextsItem()]);
  };

  const renderOptionEditor = (
    item: CustomFieldTextsItem,
    index: number,
    type: CustomFieldTextsItemType,
  ) => {
    const optionList = options.map((option) => ({ label: option, value: option }));
    if (options.length === 0) {
      return (
        <span style={{ color: '#999', fontSize: 12 }}>
          {intl.formatMessage({ id: 'content.param.no-options' })}
        </span>
      );
    }
    if (type === 'checkbox') {
      return (
        <Checkbox.Group
          disabled={disabled}
          value={(item.values || []).filter((v) => v !== '')}
          options={optionList}
          onChange={(checked) =>
            updateItem(index, { values: (checked as string[]).map(String) })
          }
        />
      );
    }
    if (type === 'radio') {
      return (
        <Radio.Group
          disabled={disabled}
          value={item.value}
          options={optionList}
          onChange={(e) => updateItem(index, { value: e.target.value })}
        />
      );
    }
    return (
      <Select
        disabled={disabled}
        style={{ width: '100%' }}
        allowClear
        value={item.value || undefined}
        options={optionList}
        onChange={(val) => updateItem(index, { value: val ?? '' })}
      />
    );
  };

  const renderFileEditor = (item: CustomFieldTextsItem, index: number, image: boolean) => {
    if (item.value) {
      return (
        <div className="ant-upload-item">
          {image ? (
            <ImageItem src={item.value} size={80} />
          ) : (
            <span className="field-value" style={{ wordBreak: 'break-all' }}>
              {item.value}
            </span>
          )}
          <span className="delete" onClick={() => updateItem(index, { value: '' })}>
            <DeleteOutlined />
          </span>
        </div>
      );
    }
    return (
      <AttachmentSelect
        open={false}
        onSelect={(row: any) => {
          if (!row) {
            return;
          }
          updateItem(index, { value: row.file_path || row.link || '' });
        }}
      >
        <div className="ant-upload-item">
          <div className="add">
            <PlusOutlined />
            <div style={{ marginTop: 8 }}>
              {intl.formatMessage({ id: 'setting.system.upload' })}
            </div>
          </div>
        </div>
      </AttachmentSelect>
    );
  };

  /**
   * images（组图）：values 是唯一真相。
   * 缩略图可左右移动/删除，上传支持一次选多张，已存在的图不重复加入。
   */
  const renderImagesEditor = (item: CustomFieldTextsItem, index: number) => {
    const images = (item.values || []).filter((v) => v !== '');
    const updateImages = (next: string[]) =>
      // value 同步成首图，避免多值删空后旧首值被后端 SyncValues 复活
      updateItem(index, { values: next, value: next[0] || '' });

    const moveImage = (imageIdx: number, offset: number) => {
      const target = imageIdx + offset;
      if (target < 0 || target >= images.length) {
        return;
      }
      const next = [...images];
      const tmp = next[imageIdx];
      next[imageIdx] = next[target];
      next[target] = tmp;
      updateImages(next);
    };

    return (
      <div className="text-images">
        {images.map((src, imageIdx) => (
          <div className="ant-upload-item" key={`${src}-${imageIdx}`}>
            <ImageItem src={src} size={80} />
            <div className="ant-upload-item-action">
              <Tag style={{ cursor: 'pointer' }} onClick={() => moveImage(imageIdx, -1)}>
                <LeftOutlined />
              </Tag>
              <Tag
                color="red"
                style={{ cursor: 'pointer' }}
                onClick={() => updateImages(images.filter((_, i) => i !== imageIdx))}
              >
                <DeleteOutlined />
              </Tag>
              <Tag style={{ cursor: 'pointer' }} onClick={() => moveImage(imageIdx, 1)}>
                <RightOutlined />
              </Tag>
            </div>
          </div>
        ))}
        <AttachmentSelect
          open={false}
          multiple={true}
          onSelect={(rows: any) => {
            if (!Array.isArray(rows) || rows.length === 0) {
              return;
            }
            const next = [...images];
            rows.forEach((row: any) => {
              const path = row?.file_path || row?.link || '';
              if (path && !next.includes(path)) {
                next.push(path);
              }
            });
            updateImages(next);
          }}
        >
          <div className="ant-upload-item">
            <div className="add">
              <PlusOutlined />
              <div style={{ marginTop: 8 }}>
                {intl.formatMessage({ id: 'setting.system.upload' })}
              </div>
            </div>
          </div>
        </AttachmentSelect>
      </div>
    );
  };

  const renderDateEditor = (
    item: CustomFieldTextsItem,
    index: number,
    type: 'date' | 'time' | 'datetime',
  ) => {
    if (type === 'time') {
      return (
        <TimePicker
          disabled={disabled}
          style={{ width: '100%' }}
          value={parseTimeValue(item.value)}
          format="HH:mm:ss"
          onChange={(val) => updateItem(index, { value: val ? val.format('HH:mm:ss') : '' })}
        />
      );
    }
    const format = type === 'date' ? 'YYYY-MM-DD' : 'YYYY-MM-DD HH:mm:ss';
    return (
      <DatePicker
        disabled={disabled}
        style={{ width: '100%' }}
        showTime={type === 'datetime'}
        value={parseDateValue(item.value)}
        format={format}
        onChange={(val) => updateItem(index, { value: val ? val.format(format) : '' })}
      />
    );
  };

  const renderColorEditor = (item: CustomFieldTextsItem, index: number) => (
    <Space.Compact style={{ width: '100%' }}>
      <ColorPicker
        disabled={disabled}
        value={item.value || ''}
        onChange={(color) => updateItem(index, { value: `#${color.toHex()}` })}
      />
      <Input
        disabled={disabled}
        value={item.value}
        placeholder="#000000"
        style={{ flex: 1 }}
        onChange={(e) => updateItem(index, { value: e.target.value })}
      />
    </Space.Compact>
  );

  const renderMultiTextEditor = (
    item: CustomFieldTextsItem,
    index: number,
    type: 'text' | 'textarea',
  ) => {
    const values = item.values && item.values.length > 0 ? item.values : [''];
    return (
      <Space size={8} direction="vertical" style={{ width: '100%' }}>
        {values.map((val, valueIdx) => {
          const onChangeValue = (next: string) => {
            const nextValues = [...values];
            nextValues[valueIdx] = next;
            updateItem(index, { values: nextValues });
          };
          return type === 'textarea' ? (
            <div key={valueIdx} style={{ display: 'flex', gap: 8 }}>
              <Input.TextArea
                disabled={disabled}
                value={val}
                autoSize={{ minRows: 1, maxRows: 6 }}
                onChange={(e) => onChangeValue(e.target.value)}
              />
              <Tag
                style={{ cursor: 'pointer', alignSelf: 'flex-start' }}
                onClick={() => {
                  if (values.length <= 1) {
                    updateItem(index, { values: [''] });
                    return;
                  }
                  updateItem(index, { values: values.filter((_, i) => i !== valueIdx) });
                }}
              >
                <DeleteOutlined />
              </Tag>
            </div>
          ) : (
            <Input
              key={valueIdx}
              disabled={disabled}
              value={val}
              prefix={values.length > 1 ? <span>{valueIdx + 1}.</span> : null}
              addonAfter={
                <Tag
                  style={{ cursor: 'pointer', marginRight: 0, border: 0, background: 'transparent' }}
                  onClick={() => {
                    if (values.length <= 1) {
                      updateItem(index, { values: [''] });
                      return;
                    }
                    updateItem(index, { values: values.filter((_, i) => i !== valueIdx) });
                  }}
                >
                  <DeleteOutlined />
                </Tag>
              }
              onChange={(e) => onChangeValue(e.target.value)}
            />
          );
        })}
        {type === 'text' && (
          <Tag
            color="blue"
            style={{ cursor: 'pointer', width: 'fit-content' }}
            onClick={() => updateItem(index, { values: [...values, ''] })}
          >
            {intl.formatMessage({ id: 'content.param.add-value' })}
          </Tag>
        )}
      </Space>
    );
  };

  const renderValueEditor = (item: CustomFieldTextsItem, index: number) => {
    const type = normalizeTextsItemType(item.type);
    switch (type) {
      case 'number': {
        const num = item.value === '' ? NaN : Number(item.value);
        return (
          <InputNumber
            disabled={disabled}
            style={{ width: '100%' }}
            value={Number.isFinite(num) ? num : undefined}
            onChange={(val) => updateItem(index, { value: val === null || val === undefined ? '' : String(val) })}
          />
        );
      }
      case 'textarea':
        return renderMultiTextEditor(item, index, 'textarea');
      case 'radio':
      case 'checkbox':
      case 'select':
        return renderOptionEditor(item, index, type);
      case 'image':
        return renderFileEditor(item, index, true);
      case 'images':
        return renderImagesEditor(item, index);
      case 'file':
        return renderFileEditor(item, index, false);
      case 'date':
      case 'time':
      case 'datetime':
        return renderDateEditor(item, index, type);
      case 'color':
        return renderColorEditor(item, index);
      default:
        return renderMultiTextEditor(item, index, 'text');
    }
  };

  return (
    <ProFormText label={label}>
      <div className="text-groups">
        <div className="text-group">
          <div className="text-key">Key</div>
          <div className="text-type">
            {intl.formatMessage({ id: 'content.param.item-type' })}
          </div>
          <div className="text-value">
            {intl.formatMessage({ id: 'content.param.item-value' })}
          </div>
          <div className="text-action" />
        </div>
        {items.map((item, index) => {
          const type = normalizeTextsItemType(item.type);
          return (
            <div className="text-group" key={index}>
              <div className="text-key">
                <Input
                  disabled={disabled}
                  value={item.key}
                  onChange={(e) => updateItem(index, { key: e.target.value })}
                />
              </div>
              <div className="text-type">
                <Select
                  disabled={disabled}
                  style={{ width: '100%' }}
                  value={type}
                  options={CUSTOM_FIELD_TEXTS_TYPES.map((option) => ({
                    value: option.value,
                    label: intl.formatMessage({ id: option.labelId }),
                  }))}
                  onChange={(val) => {
                    const next = items.map((it, i) =>
                      i === index
                        ? changeTextsItemType(it, val as CustomFieldTextsItemType)
                        : it,
                    );
                    emit(next);
                  }}
                />
              </div>
              <div className="text-value">{renderValueEditor(item, index)}</div>
              <div className="text-action">
                <Tag
                  style={{ cursor: 'pointer' }}
                  onClick={() => moveItem(index, -1)}
                  title="up"
                >
                  <UpOutlined />
                </Tag>
                <Tag
                  style={{ cursor: 'pointer' }}
                  onClick={() => moveItem(index, 1)}
                  title="down"
                >
                  <DownOutlined />
                </Tag>
                <Popconfirm
                  title={intl.formatMessage({ id: 'content.param.delete-item' })}
                  onConfirm={() => removeItem(index)}
                >
                  <Tag color="red" style={{ cursor: 'pointer' }}>
                    <DeleteOutlined />
                  </Tag>
                </Popconfirm>
              </div>
            </div>
          );
        })}
        <div className="text-group">
          <div className="text-key">
            <Tag
              color="blue"
              className="add-line"
              style={{ cursor: 'pointer' }}
              onClick={addItem}
            >
              {intl.formatMessage({ id: 'content.param.add-line' })}
            </Tag>
          </div>
        </div>
      </div>
    </ProFormText>
  );
};

export default CustomFieldTextsEditor;
