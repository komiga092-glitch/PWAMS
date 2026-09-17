"use strict";
const CACHE_NAME = "pwams-static-v2";
const OFFLINE_SHELL = "/offline.html";
const STATIC_ASSETS = [
    "/static/css/app.css",
    "/static/js/app.js",
    "/static/js/offline/register.js",
    "/static/js/offline/i18n.js",
    "/static/js/offline/csrf.js",
    "/static/js/offline/connectivity.js",
    "/static/js/offline/db.js",
    "/static/js/offline/mutations.js",
    "/static/js/offline/sync.js",
    "/static/js/offline/conflicts.js",
    "/static/js/offline/pull.js",
    "/static/js/offline/pages.js",
    "/static/js/offline/session.js",
    "/static/js/offline/media.js",
    "/static/js/offline-data.js",
    "/static/js/offline/service-worker.js",
    "/static/manifest.webmanifest",
    OFFLINE_SHELL,
];
const worker = self;
worker.addEventListener("install", (event) => {
    event.waitUntil(caches
        .open(CACHE_NAME)
        .then((cache) => cache.addAll(STATIC_ASSETS))
        .then(() => worker.skipWaiting()));
});
worker.addEventListener("activate", (event) => {
    event.waitUntil(caches
        .keys()
        .then((keys) => Promise.all(keys
        .filter((key) => key !== CACHE_NAME)
        .map((key) => caches.delete(key))))
        .then(() => worker.clients.claim()));
});
worker.addEventListener("fetch", (event) => {
    const request = event.request;
    const url = new URL(request.url);
    if (request.method !== "GET" || url.origin !== worker.location.origin)
        return;
    if (url.pathname.startsWith("/static/")) {
        event.respondWith(fetch(request).catch(() => caches
            .match(request)
            .then((response) => response ?? new Response("Offline", { status: 503 }))));
        return;
    }
    if (request.mode === "navigate") {
        event.respondWith(fetch(request).catch(() => caches.match(OFFLINE_SHELL).then((response) => response ??
            new Response("Offline", {
                status: 503,
                headers: { "Content-Type": "text/html" },
            }))));
    }
});
worker.addEventListener("sync", (event) => {
    if (event.tag === "pwams-background-sync") {
        event.waitUntil(new Promise((resolve) => {
            worker.clients.matchAll().then((clientList) => {
                clientList.forEach((client) => {
                    client.postMessage({ type: "PWAMS_SYNC_REQUESTED" });
                });
                resolve();
            });
        }));
    }
});
//# sourceMappingURL=service-worker.js.map