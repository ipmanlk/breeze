// Plume service worker; receives Web Push events and displays OS-level
// notifications even when the app tab is closed. Registered from main.ts.
//
// Push payloads arrive aes128gcm-encrypted; the browser decrypts them before
// dispatching `push` events, so event.data.json() gives us the PushPayload the
// backend sent.

self.addEventListener("install", (event) => {
  // Activate immediately so the first push subscription works without a reload.
  self.skipWaiting();
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  let payload = { title: "Plume", body: "" };
  try {
    if (event.data) {
      payload = event.data.json();
    }
  } catch {
    // Some push services send a plain-text payload; fall back to it.
    if (event.data) {
      payload = { title: "Plume", body: event.data.text() };
    }
  }
  const { title, body, link, tag } = payload;
  const options = {
    body: body || "",
    tag: tag || "plume-notification",
    // Reuse the same notification tag so a flood of mentions collapses.
    renotify: true,
    data: { link: link || "" },
    icon: "/favicon.svg",
    badge: "/favicon.svg",
  };
  event.waitUntil(
    self.registration.showNotification(title || "Plume", options),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const link = event.notification.data && event.notification.data.link;
  event.waitUntil(
    (async () => {
      const allClients = await self.clients.matchAll({
        type: "window",
        includeUncontrolled: true,
      });
      // Focus an existing tab if one is open to Plume.
      for (const client of allClients) {
        if (client.url.includes(self.location.origin)) {
          if (link) {
            client.postMessage({ type: "navigate", link });
          }
          return client.focus();
        }
      }
      // Otherwise open a new tab.
      if (self.clients.openWindow) {
        return self.clients.openWindow(link || "/");
      }
      return null;
    })(),
  );
});
