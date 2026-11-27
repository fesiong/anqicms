export type PlaceSelectionItem = {
  country_code?: string;
} | null | undefined;

// PlaceCascader 的 required 规则不能只看“有没有对象”：
// 清空选择时 handleCascaderChange 仍会返回 {country_code:'',country:''}，
// 在 antd 的 required 眼里这是真值，于是“未选发货地也能保存”。
// 这里按 country_code 判定，单选与多选共用。
export function hasSelectedPlace(
  value: PlaceSelectionItem | PlaceSelectionItem[],
  multiple: boolean,
): boolean {
  if (multiple) {
    return (
      Array.isArray(value) &&
      value.length > 0 &&
      value.every((item) => !!item?.country_code)
    );
  }
  return !Array.isArray(value) && !!value?.country_code;
}
