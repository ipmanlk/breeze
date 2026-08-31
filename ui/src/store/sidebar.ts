import { signal } from "@preact/signals-core";

const STORAGE_KEY = "plume_sidebar_state";
const MOBILE_BREAKPOINT = 768;

function getStored(): boolean {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw !== "false";
  } catch {
    return true;
  }
}

function isMobile(): boolean {
  return window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 0.02}px)`)
    .matches;
}

export const sidebarOpen = signal<boolean>(getStored());
export const sidebarMobileOpen = signal<boolean>(false);
export const sidebarIsMobile = signal<boolean>(isMobile());

let mq: MediaQueryList | null = null;
let keydownHandler: ((e: KeyboardEvent) => void) | null = null;

function handleChange(e: MediaQueryListEvent | MediaQueryList): void {
  sidebarIsMobile.value = e.matches;
}

export function initSidebar(): void {
  mq = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 0.02}px)`);
  mq.addEventListener("change", handleChange);
  handleChange(mq);

  if (!keydownHandler) {
    keydownHandler = (e: KeyboardEvent) => {
      if (e.key === "b" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        toggleSidebar();
      }
    };
    window.addEventListener("keydown", keydownHandler);
  }
}

export function cleanupSidebar(): void {
  mq?.removeEventListener("change", handleChange);
  mq = null;
  if (keydownHandler) {
    window.removeEventListener("keydown", keydownHandler);
    keydownHandler = null;
  }
}

export function toggleSidebar(): void {
  if (sidebarIsMobile.value) {
    sidebarMobileOpen.value = !sidebarMobileOpen.value;
  } else {
    const next = !sidebarOpen.value;
    sidebarOpen.value = next;
    try {
      localStorage.setItem(STORAGE_KEY, String(next));
    } catch {
      // ignore
    }
  }
}

export function setSidebarOpen(open: boolean): void {
  if (sidebarIsMobile.value) {
    sidebarMobileOpen.value = open;
  } else {
    sidebarOpen.value = open;
    try {
      localStorage.setItem(STORAGE_KEY, String(open));
    } catch {
      // ignore
    }
  }
}
