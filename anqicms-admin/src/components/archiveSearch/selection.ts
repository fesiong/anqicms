export type SelectedArchive = { id: number; title: string };

export function mergeSelectedArchives(
  previous: SelectedArchive[],
  ids: number[],
  rows: (SelectedArchive | undefined)[],
): SelectedArchive[] {
  const loaded = rows.filter(
    (row): row is SelectedArchive => row !== undefined,
  );
  const byId = new Map(previous.concat(loaded).map((row) => [row.id, row]));
  return ids.map((id) => byId.get(id) || { id, title: `#${id}` });
}

export async function loadSelectedArchives(
  ids: number[],
  getDetail: (params: { id: number }) => Promise<any>,
): Promise<SelectedArchive[]> {
  const result: SelectedArchive[] = [];
  for (let offset = 0; offset < ids.length; offset += 5) {
    const rows = await Promise.all(
      ids.slice(offset, offset + 5).map(async (id) => {
        try {
          const response = await getDetail({ id });
          return { id, title: response.data?.title || `#${id}` };
        } catch {
          return { id, title: `#${id}` };
        }
      }),
    );
    result.push(...rows);
  }
  return result;
}
