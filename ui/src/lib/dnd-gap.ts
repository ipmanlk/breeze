/**
 * DnD gap computation: shared by kanban-board, status-settings, and
 * conversation-dnd. Finds the insertion index and Y position for a
 * drag indicator based on the cursor's clientY relative to draggable
 * children identified by a data attribute.
 */

export function computeGap(
  container: Element,
  clientY: number,
  dataAttr: string,
  excludeId?: string,
): number {
  const items = Array.from(
    container.querySelectorAll<HTMLElement>(`[data-${dataAttr}]`),
  );
  const filtered = excludeId
    ? items.filter((el) => el.getAttribute(`data-${dataAttr}`) !== excludeId)
    : items;
  if (filtered.length === 0) return 0;
  for (let i = 0; i < filtered.length; i++) {
    const rect = filtered[i].getBoundingClientRect();
    if (clientY < rect.top + rect.height / 2) return i;
  }
  return filtered.length;
}

export function computeGapY(
  container: Element,
  gapIndex: number,
  dataAttr: string,
  excludeId?: string,
  emptyY = 0,
): number {
  const containerRect = container.getBoundingClientRect();
  const items = Array.from(
    container.querySelectorAll<HTMLElement>(`[data-${dataAttr}]`),
  );
  const filtered = excludeId
    ? items.filter((el) => el.getAttribute(`data-${dataAttr}`) !== excludeId)
    : items;
  if (filtered.length === 0) return emptyY;
  if (gapIndex === 0) {
    return filtered[0].getBoundingClientRect().top - containerRect.top;
  }
  if (gapIndex >= filtered.length) {
    return filtered[filtered.length - 1].getBoundingClientRect().bottom -
      containerRect.top;
  }
  const prev = filtered[gapIndex - 1].getBoundingClientRect();
  const next = filtered[gapIndex].getBoundingClientRect();
  return (prev.bottom + next.top) / 2 - containerRect.top;
}
