/**
 * Browser push notification manager.
 *
 * Registers the service worker, subscribes to Web Push via the Push API,
 * and syncs the subscription to the backend (which stores the per-user
 * endpoint + keys so it can send encrypted pushes).
 *
 * Opt-in is gated on the user's `desktop_notifications` preference. When the
 * server has no VAPID keys configured (`enabled: false` on the public-key
 * endpoint), this module is a no-op: the existing in-page Notification API
 * (wired in app-shell) still covers the tab-open case.
 */

import {
  deletePushSubscribe,
  getPushVapidPublicKey,
  postPushSubscribe,
} from "@/api";
import { desktopNotificationsEnabled } from "./preferences";
import { auth } from "./auth";

let registered = false;
let swRegistration: ServiceWorkerRegistration | null = null;

/** Register the service worker once. Safe to call repeatedly. */
export async function ensureServiceWorker(): Promise<
  ServiceWorkerRegistration | null
> {
  if (!("serviceWorker" in navigator)) return null;
  try {
    swRegistration = await navigator.serviceWorker.register("/sw.js", {
      scope: "/",
    });
    return swRegistration;
  } catch {
    return null;
  }
}

/**
 * Attempt to set up a push subscription for the current user if:
 *  - the service worker is registered,
 *  - the server has VAPID configured,
 *  - the user has desktop notifications enabled.
 *
 * No-ops silently otherwise. Best-effort: errors are swallowed.
 */
export async function initPush(): Promise<void> {
  if (registered) return;
  if (!auth.value.isAuthenticated) return;
  if (!desktopNotificationsEnabled()) return;

  const reg = await ensureServiceWorker();
  if (!reg) return;

  // Fetch the server's VAPID public key.
  let vapidKey: string;
  try {
    const { data } = await getPushVapidPublicKey({ throwOnError: true });
    const resp = data;
    if (!resp.enabled || !resp.public_key) return;
    vapidKey = resp.public_key;
  } catch {
    return;
  }

  // Check if a subscription already exists.
  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    try {
      sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(vapidKey)
          .buffer as ArrayBuffer,
      });
    } catch {
      // Permission denied or browser blocked: give up silently.
      return;
    }
  }

  // Sync to backend.
  try {
    await postPushSubscribe({
      body: {
        endpoint: sub.endpoint,
        p256dh: arrayBufferToB64(sub.getKey("p256dh")),
        auth: arrayBufferToB64(sub.getKey("auth")),
      },
      throwOnError: true,
    });
    registered = true;
  } catch {
    // Non-fatal: the subscription exists locally and will retry on next load.
  }
}

/** Unsubscribe the current browser from push + notify the backend. */
export async function unsubscribePush(): Promise<void> {
  if (!swRegistration) return;
  const sub = await swRegistration.pushManager.getSubscription();
  if (sub) {
    const endpoint = sub.endpoint;
    await sub.unsubscribe().catch(() => {});
    await deletePushSubscribe({
      body: { endpoint, p256dh: "", auth: "" },
    }).catch(() => {});
  }
  registered = false;
}

// helpers
/** Convert a VAPID base64url public key to a Uint8Array for pushManager. */
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const output = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) {
    output[i] = raw.charCodeAt(i);
  }
  return output;
}

/** Convert an ArrayBuffer (from push subscription keys) to base64url string. */
function arrayBufferToB64(buf: ArrayBuffer | null): string {
  if (!buf) return "";
  const bytes = new Uint8Array(buf);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(
    /=+$/,
    "",
  );
}
