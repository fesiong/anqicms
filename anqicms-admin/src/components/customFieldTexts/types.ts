/**
 * texts 类型自定义字段（文本列表）的条目模型与纯逻辑。
 *
 * 与后端 config.CustomFieldTexts 一一对应：
 *   { key, value, values, type }
 *
 * 历史数据的条目没有 type，只有 key + value/values，按 text 处理；
 * 写回时 text 类型的 type 会被省略，保证旧数据不会被塞进新字段。
 */

export type CustomFieldTextsItemType =
  | 'text'
  | 'number'
  | 'textarea'
  | 'radio'
  | 'checkbox'
  | 'select'
  | 'image'
  | 'images'
  | 'file'
  | 'date'
  | 'time'
  | 'datetime'
  | 'color';

export interface CustomFieldTextsItem {
  key: string;
  value: string;
  values: string[];
  /** 空串表示 text（与历史数据保持一致） */
  type: CustomFieldTextsItemType | '';
}

export interface CustomFieldTextsTypeOption {
  value: CustomFieldTextsItemType;
  labelId: string;
}

/** 条目可选类型，与后端 CustomFieldTextsTypes 白名单严格一致 */
export const CUSTOM_FIELD_TEXTS_TYPES: CustomFieldTextsTypeOption[] = [
  { value: 'text', labelId: 'content.module.field.type.text' },
  { value: 'number', labelId: 'content.module.field.type.number' },
  { value: 'textarea', labelId: 'content.module.field.type.textarea' },
  { value: 'radio', labelId: 'content.module.field.type.radio' },
  { value: 'checkbox', labelId: 'content.module.field.type.checkbox' },
  { value: 'select', labelId: 'content.module.field.type.select' },
  { value: 'image', labelId: 'content.module.field.type.image' },
  { value: 'images', labelId: 'content.module.field.type.images' },
  { value: 'file', labelId: 'content.module.field.type.file' },
  { value: 'date', labelId: 'content.module.field.type.date' },
  { value: 'time', labelId: 'content.module.field.type.time' },
  { value: 'datetime', labelId: 'content.module.field.type.datetime' },
  { value: 'color', labelId: 'content.module.field.type.color' },
];

const TEXT_TYPE_VALUES = CUSTOM_FIELD_TEXTS_TYPES.map((item) => item.value);

/** 把任意 type 归一化成 UI 可用的类型：空/非法一律按 text */
export function normalizeTextsItemType(type?: string): CustomFieldTextsItemType {
  const value = (type || '').trim().toLowerCase();
  return (TEXT_TYPE_VALUES.includes(value as CustomFieldTextsItemType)
    ? value
    : 'text') as CustomFieldTextsItemType;
}

/** 是否可以有多个值：text/textarea 沿用历史的多行，checkbox 是选项多选，images 是组图 */
export function isMultiValueType(type?: string): boolean {
  const value = normalizeTextsItemType(type);
  return (
    value === 'text' ||
    value === 'textarea' ||
    value === 'checkbox' ||
    value === 'images'
  );
}

/** 是否为「值本身就是短串」的多值类型：展示时用逗号而不是换行 */
export function isCommaMultiValueType(type?: string): boolean {
  const value = normalizeTextsItemType(type);
  return value === 'checkbox' || value === 'images';
}

/** 是否从字段配置的 content（每行一个选项）里取值 */
export function isOptionType(type?: string): boolean {
  const value = normalizeTextsItemType(type);
  return value === 'radio' || value === 'checkbox' || value === 'select';
}

/** 字段配置里的选项：按行拆分、去空白、去重 */
export function parseTextsOptions(content?: string): string[] {
  const result: string[] = [];
  (content || '').split('\n').forEach((line) => {
    const value = line.trim();
    if (value && !result.includes(value)) {
      result.push(value);
    }
  });
  return result;
}

function toText(value: any): string {
  if (value === null || value === undefined) {
    return '';
  }
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return '';
}

/**
 * 把任意历史形态的 texts 值转成条目数组。
 * 支持：JSON 字符串 / CustomField 包装体 { value: [...] } / 数组。
 */
export function normalizeTextsItems(value: any): CustomFieldTextsItem[] {
  let list = value;
  if (typeof list === 'string') {
    try {
      list = JSON.parse(list);
    } catch (e) {
      return [];
    }
  }
  if (list && !Array.isArray(list) && typeof list === 'object' && 'value' in list) {
    list = (list as any).value;
  }
  if (!Array.isArray(list)) {
    return [];
  }
  const result: CustomFieldTextsItem[] = [];
  list.forEach((raw: any) => {
    if (!raw || typeof raw !== 'object') {
      return;
    }
    const type = normalizeTextsItemType(raw.type);
    let itemValue = toText(raw.value);
    let values = Array.isArray(raw.values) ? raw.values.map(toText) : [];
    if (isMultiValueType(type)) {
      if (values.length === 0) {
        values = itemValue !== '' ? [itemValue] : [''];
      }
    } else if (itemValue === '') {
      // 单值类型没有 value 时，用旧的 values 补一个，避免升级后看起来是空的
      itemValue = values.find((v: string) => v !== '') || '';
    }
    result.push({
      key: toText(raw.key),
      value: itemValue,
      values,
      type: type === 'text' ? '' : type,
    });
  });
  return result;
}

/** 新建一个空条目 */
export function createTextsItem(
  type: CustomFieldTextsItemType = 'text',
): CustomFieldTextsItem {
  return {
    key: '',
    value: '',
    values: isMultiValueType(type) ? [''] : [],
    type: type === 'text' ? '' : type,
  };
}

/**
 * 切换类型时补齐另一侧的值：不销毁用户已经填的内容，
 * 只保证当前类型对应的字段有值可用。
 */
export function changeTextsItemType(
  item: CustomFieldTextsItem,
  type: CustomFieldTextsItemType,
): CustomFieldTextsItem {
  const next: CustomFieldTextsItem = {
    ...item,
    values: [...(item.values || [])],
    type: type === 'text' ? '' : type,
  };
  if (isMultiValueType(type)) {
    if (next.values.length === 0) {
      next.values = next.value !== '' ? [next.value] : [''];
    }
  } else if (next.value === '' && next.values.length > 0) {
    next.value = next.values.find((v) => v !== '') || '';
  }
  return next;
}

/** 序列化成后端认的载荷：text 不写 type 字段，保持与历史数据同形 */
export function serializeTextsItems(items: CustomFieldTextsItem[]): any[] {
  return (items || []).map((item) => {
    const type = normalizeTextsItemType(item.type);
    const values = (item.values || []).slice();
    // 多值类型以 values 为唯一真相，value 取第一个非空值（与后端 SyncValues 一致，
    // 保证历史消费方读 value 也能拿到内容）；单值类型以 value 为准。
    // 这里刻意不回落 item.value：组图/多行删空后若还带着旧的首值，
    // 后端 SyncValues 会把已删除的图重新派生回 values。
    const firstFilled = values.find((v: string) => v !== '') || '';
    const payload: any = {
      key: item.key || '',
      value: isMultiValueType(type) ? firstFilled : item.value || '',
      // 原样保留 values（含空行）：空行是编辑器里「刚添加待填写」的占位，
      // 过滤掉会让用户点一下「添加值」看不到新行。空值由后端校验时忽略。
      values,
    };
    if (type !== 'text') {
      payload.type = type;
    }
    return payload;
  });
}

/** 提交用：序列化并丢掉完全空白的行（无 key 且没有任何值） */
export function serializeFilledTextsItems(items: CustomFieldTextsItem[]): any[] {
  return serializeTextsItems(items).filter(
    (item) =>
      item.key !== '' ||
      item.value !== '' ||
      (item.values || []).some((v: string) => v !== ''),
  );
}

/** 条目用于展示的文本（checkbox/images 用逗号，其余多值用换行） */
export function textsItemDisplayText(item: any): string {
  const type = normalizeTextsItemType(item?.type);
  const value = toText(item?.value);
  const values = Array.isArray(item?.values) ? item.values.map((v: any) => toText(v)) : [];
  if (isMultiValueType(type)) {
    const filled = values.filter((v: string) => v !== '');
    if (filled.length > 0) {
      return filled.join(isCommaMultiValueType(type) ? ', ' : '\n');
    }
    return value;
  }
  return value;
}
