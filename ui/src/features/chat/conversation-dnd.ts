import {
  draggable,
  dropTargetForElements,
  monitorForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { identify } from "@/lib/sdk-helpers";
import { generateKeyBetween } from "@/lib/lexorank";
import { computeGap, computeGapY } from "@/lib/dnd-gap";
import { chatApi } from "./api";
import { conversationList } from "./store";
import type { Conversation } from "./types";

/* Types */

export interface ConvDragData {
  type: "category" | "channel";
  id: string;
  parent_id?: string;
  position_key: string;
}

/* Key helpers */

function compareKeys(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** Sorted siblings for a given parent (channels+voice) or all categories. */
function siblingsFor(
  list: Conversation[],
  parentId: string | undefined,
): Conversation[] {
  if (parentId === undefined) {
    return list
      .filter((c) => c.type === "category")
      .sort((a, b) => compareKeys(a.position_key, b.position_key));
  }
  return list
    .filter(
      (c) =>
        (c.parent_id ?? undefined) === parentId &&
        (c.type === "channel" || c.type === "voice"),
    )
    .sort((a, b) => compareKeys(a.position_key, b.position_key));
}

/** Compute a new position_key for inserting at `gapIndex` among `siblings`. */
function keyForGap(
  siblings: Conversation[],
  gapIndex: number,
): string {
  const prev = gapIndex > 0 ? siblings[gapIndex - 1].position_key : null;
  const next = gapIndex < siblings.length
    ? siblings[gapIndex].position_key
    : null;
  return generateKeyBetween(prev, next);
}

function computeCatGap(
  container: HTMLElement,
  clientY: number,
  excludeId?: string,
): number {
  const rows = Array.from(
    container.querySelectorAll<HTMLElement>(".ws-category"),
  );
  const filtered = excludeId
    ? rows.filter((el) =>
      el.querySelector(".ws-cat-header")?.getAttribute("data-category-id") !==
        excludeId
    )
    : rows;
  if (filtered.length === 0) return 0;
  for (const r of filtered) {
    const rect = r.getBoundingClientRect();
    if (clientY < rect.top + rect.height / 2) {
      return filtered.indexOf(r);
    }
  }
  return filtered.length;
}

function computeCatGapY(
  container: HTMLElement,
  gapIndex: number,
  excludeId?: string,
): number {
  const containerRect = container.getBoundingClientRect();
  const rows = Array.from(
    container.querySelectorAll<HTMLElement>(".ws-category"),
  );
  const filtered = excludeId
    ? rows.filter((el) =>
      el.querySelector(".ws-cat-header")?.getAttribute("data-category-id") !==
        excludeId
    )
    : rows;
  if (filtered.length === 0) return 0;
  if (gapIndex === 0) {
    return filtered[0].getBoundingClientRect().top - containerRect.top;
  }
  if (gapIndex >= filtered.length) {
    return (
      filtered[filtered.length - 1].getBoundingClientRect().bottom -
      containerRect.top
    );
  }
  const prev = filtered[gapIndex - 1].getBoundingClientRect();
  const next = filtered[gapIndex].getBoundingClientRect();
  return (prev.bottom + next.top) / 2 - containerRect.top;
}

/* Persist */

async function persistMove(
  src: ConvDragData,
  destParentId: string | undefined,
  newKey: string,
) {
  const prevList = conversationList.value;
  conversationList.value = prevList.map((c) =>
    c.id === src.id
      ? { ...c, parent_id: destParentId, position_key: newKey }
      : c
  );
  try {
    await chatApi.updatePosition(src.id, {
      parent_id: destParentId,
      position_key: newKey,
    });
  } catch {
    conversationList.value = prevList;
  }
}

/* Central monitor */

let _monitorCleanup: (() => void) | null = null;

export function startConversationMonitor() {
  _monitorCleanup?.();
  _monitorCleanup = monitorForElements({
    canMonitor: ({ source }) => {
      const data = source.data as Record<string, unknown>;
      return data.type === "category" || data.type === "channel";
    },
    onDrop: async ({ source, location }) => {
      const src = identify<ConvDragData>(source.data);
      if (!src.id) return;

      const dropTargets = location.current.dropTargets;
      // Innermost (most specific) drop target wins.
      const inner = dropTargets[0];
      if (!inner) return;

      const innerData = inner.data as Record<string, unknown>;
      const innerEl = inner.element as HTMLElement;
      const current = conversationList.value;

      // Channel dropped inside a `.ws-channels` container → reorder within /
      // move between categories. `innerData.cat` is the category id (or "" for
      // uncategorized) set by `setupChannelDropTarget`.
      if (src.type === "channel" && innerData.cat !== undefined) {
        const catAttr = innerData.cat as string;
        const destParentId = catAttr === "" ? undefined : catAttr;
        const siblings = siblingsFor(current, destParentId).filter(
          (c) => c.id !== src.id,
        );
        const gap = computeGap(
          innerEl,
          location.current.input.clientY,
          "channel-id",
          src.id,
        );
        const clamped = Math.max(0, Math.min(gap, siblings.length));
        const newKey = keyForGap(siblings, clamped);
        if (src.parent_id === destParentId && src.position_key === newKey) {
          return;
        }
        await persistMove(src, destParentId, newKey);
        return;
      }

      // Channel dropped on a category header → move into that category (end)
      if (src.type === "channel" && innerData.type === "category") {
        const catId = innerData.id as string;
        const siblings = siblingsFor(current, catId).filter(
          (c) => c.id !== src.id,
        );
        const newKey = keyForGap(siblings, siblings.length);
        if (src.parent_id === catId && src.position_key === newKey) return;
        await persistMove(src, catId, newKey);
        return;
      }

      // Category dropped on `.ws-cats-wrap` → reorder categories
      if (src.type === "category" && innerData.cat === "__cats__") {
        const siblings = siblingsFor(current, undefined).filter(
          (c) => c.id !== src.id,
        );
        const gap = computeCatGap(
          innerEl,
          location.current.input.clientY,
          src.id,
        );
        const clamped = Math.max(0, Math.min(gap, siblings.length));
        const newKey = keyForGap(siblings, clamped);
        if (src.position_key === newKey) return;
        await persistMove(src, undefined, newKey);
        return;
      }

      // Channel dropped on `.ws-cats-wrap` (but not on a `.ws-channels`) →
      // move to uncategorized (end). This happens when dropping in the gap
      // between categories or on the wrapper background.
      if (src.type === "channel" && innerData.cat === "__cats__") {
        const unparented = current
          .filter(
            (c) =>
              !c.parent_id &&
              (c.type === "channel" || c.type === "voice") &&
              c.id !== src.id,
          )
          .sort((a, b) => compareKeys(a.position_key, b.position_key));
        const newKey = keyForGap(unparented, unparented.length);
        if (src.parent_id === undefined && src.position_key === newKey) return;
        await persistMove(src, undefined, newKey);
        return;
      }
    },
  });
}

export function stopConversationMonitor() {
  _monitorCleanup?.();
  _monitorCleanup = null;
}

/* Setup draggable on a channel item */

export function setupChannelDraggable(
  el: HTMLElement,
  conv: Conversation,
): () => void {
  const data: ConvDragData = {
    type: "channel",
    id: conv.id,
    parent_id: conv.parent_id,
    position_key: conv.position_key,
  };
  return draggable({
    element: el,
    getInitialData: () => identify<Record<string, unknown>>(data),
    onDragStart: () => el.setAttribute("data-dragging", ""),
    onDrop: () => el.removeAttribute("data-dragging"),
  });
}

/* Setup draggable on a category header */

export function setupCategoryDraggable(
  el: HTMLElement,
  conv: Conversation,
): () => void {
  const data: ConvDragData = {
    type: "category",
    id: conv.id,
    parent_id: conv.parent_id,
    position_key: conv.position_key,
  };
  return draggable({
    element: el,
    getInitialData: () => identify<Record<string, unknown>>(data),
    onDragStart: () => el.setAttribute("data-dragging", ""),
    onDrop: () => el.removeAttribute("data-dragging"),
  });
}

/**
 * Category header drop target: accepts channel drops (move channel into this
 * category). Carries `{ type: "category", id }` so the central monitor can
 * identify the destination. Visual feedback is just a highlight on the header.
 */
export function setupCategoryHeaderDropTarget(
  el: HTMLElement,
  conv: Conversation,
): () => void {
  return dropTargetForElements({
    element: el,
    canDrop: ({ source }) => {
      const data = source.data as Record<string, unknown>;
      return data.type === "channel";
    },
    getData: () =>
      identify<Record<string, unknown>>({ type: "category", id: conv.id }),
    onDragEnter: () => el.setAttribute("data-cat-over", ""),
    onDragLeave: () => el.removeAttribute("data-cat-over"),
    onDrop: () => el.removeAttribute("data-cat-over"),
  });
}

/* Drop targets */

/**
 * Channel list container drop target. Carries `cat` (category id or "" for
 * uncategorized) so the central monitor knows the destination. Does NOT handle
 * the drop itself: the central monitor reads `dropTargets[0].data`.
 */
export function setupChannelDropTarget(
  container: HTMLElement,
  indicator: HTMLElement,
  categoryId: string | null,
): () => void {
  const cat = categoryId ?? "";
  const updateIndicator = (clientY: number, excludeId?: string) => {
    const gap = computeGap(container, clientY, "channel-id", excludeId);
    const y = computeGapY(container, gap, "channel-id", excludeId);
    indicator.style.top = `${y - 2}px`;
    indicator.style.display = "flex";
  };
  const hideIndicator = () => {
    indicator.style.display = "none";
  };

  return dropTargetForElements({
    element: container,
    canDrop: ({ source }) => {
      const data = source.data as Record<string, unknown>;
      return data.type === "channel";
    },
    getData: () => identify<Record<string, unknown>>({ cat }),
    onDragEnter: ({ source, location }) => {
      container.setAttribute("data-over", "");
      updateIndicator(
        location.current.input.clientY,
        (source.data as Record<string, unknown>).id as string | undefined,
      );
    },
    onDrag: ({ source, location }) => {
      updateIndicator(
        location.current.input.clientY,
        (source.data as Record<string, unknown>).id as string | undefined,
      );
    },
    onDragLeave: () => {
      container.removeAttribute("data-over");
      hideIndicator();
    },
    onDrop: () => {
      container.removeAttribute("data-over");
      hideIndicator();
    },
  });
}

/**
 * Categories wrapper drop target. Carries `cat: "__cats__"` so the central
 * monitor knows this is the category-reorder / move-to-uncategorized zone.
 * Does NOT handle the drop itself.
 */
export function setupCategoryDropTarget(
  container: HTMLElement,
  indicator: HTMLElement,
): () => void {
  const updateIndicator = (clientY: number, excludeId?: string) => {
    const gap = computeCatGap(container, clientY, excludeId);
    const y = computeCatGapY(container, gap, excludeId);
    indicator.style.top = `${y - 2}px`;
    indicator.style.display = "flex";
  };
  const hideIndicator = () => {
    indicator.style.display = "none";
  };

  return dropTargetForElements({
    element: container,
    canDrop: ({ source }) => {
      const data = source.data as Record<string, unknown>;
      return data.type === "category" || data.type === "channel";
    },
    getData: () => identify<Record<string, unknown>>({ cat: "__cats__" }),
    onDragEnter: ({ source, location }) => {
      container.setAttribute("data-over", "");
      const srcData = source.data as Record<string, unknown>;
      if (srcData.type === "category") {
        updateIndicator(
          location.current.input.clientY,
          srcData.id as string | undefined,
        );
      }
    },
    onDrag: ({ source, location }) => {
      const srcData = source.data as Record<string, unknown>;
      if (srcData.type === "category") {
        updateIndicator(
          location.current.input.clientY,
          srcData.id as string | undefined,
        );
      }
    },
    onDragLeave: () => {
      container.removeAttribute("data-over");
      hideIndicator();
    },
    onDrop: () => {
      container.removeAttribute("data-over");
      hideIndicator();
    },
  });
}
