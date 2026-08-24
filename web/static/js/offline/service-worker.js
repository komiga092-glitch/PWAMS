"use strict";
const CACHE_NAME = "pwams-static-v1";
const STATIC_ASSETS = [
    "/static/css/app.css",
    "/static/js/app.js",
    "/static/js/offline/register.js",
    "/static/js/offline/connectivity.js",
    "/static/js/offline/db.js",
    "/static/js/offline/mutations.js",
    "/static/js/offline/sync.js",
    "/static/js/offline/conflicts.js",
    "/static/js/offline/pull.js",
    "/static/js/offline/pages.js",
    "/static/js/offline-data.js",
    "/static/js/offline/service-worker.js",
    "/static/manifest.webmanifest",
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
    if (request.method !== "GET" ||
        url.origin !== worker.location.origin ||
        !url.pathname.startsWith("/static/"))
        return;
    event.respondWith(fetch(request).catch(() => caches
        .match(request)
        .then((response) => response ?? new Response("Offline", { status: 503 }))));
});
//# sourceMappingURL=service-worker.js.map