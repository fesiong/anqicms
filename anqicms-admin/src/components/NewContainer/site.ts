export type WebsiteTab = {
  key: number;
  tab: string;
  is_main: boolean;
};

export function normalizeSites(items: any[]): WebsiteTab[] {
  return (items || [])
    .map((item) => ({
      key: Number(item.id),
      tab: item.name,
      is_main: Boolean(item.is_main),
    }))
    .filter((item) => Number.isFinite(item.key));
}

export function resolveSiteSelection(
  sites: WebsiteTab[],
  storedSiteId?: any,
): { activeKey: string; isSubsite: boolean } {
  const site = sites.find((item) => String(item.key) === String(storedSiteId ?? ''));
  const selected = site || sites[0];
  return {
    activeKey: selected ? String(selected.key) : '',
    isSubsite: selected ? !selected.is_main : false,
  };
}
