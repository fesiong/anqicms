// 后台列表接口统一带 total；Go 的 nil slice 会序列化成 data:null。
// ProTable 在 total=0 且 data=null 时会求值 null.length，
// 抛 "Cannot read properties of null (reading 'length')"，并且 total 不会刷新为 0。
//
// 只在“带数字 total 的列表信封”上兜底，避免误改详情接口的 data:null。
export function normalizeListData<T = any>(payload: T): T {
  if (
    payload &&
    typeof payload === 'object' &&
    (payload as any).data === null &&
    typeof (payload as any).total === 'number'
  ) {
    return { ...(payload as any), data: [] } as T;
  }
  return payload;
}
