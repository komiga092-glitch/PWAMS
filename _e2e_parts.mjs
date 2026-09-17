// _e2e_parts.mjs — shared fixtures + tiny static server for the PWAMS offline E2E
export const ROOT = "e:\\PWAMS";
export const STATIC = ROOT + "\\web\\static";
export const port = 18923;
export const base = `http://127.0.0.1:${port}`;

export const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".webmanifest": "application/manifest+json; charset=utf-8",
  ".map": "application/json; charset=utf-8",
  ".ts": "text/plain; charset=utf-8",
  ".png": "image/png",
  ".ico": "image/x-icon",
  ".svg": "image/svg+xml",
};

export function pageShell(title, mainHtml) {
  const head = "<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"UTF-8\"><title>"
    + title + "</title><link rel=\"stylesheet\" href=\"/static/css/app.css\">"
    + "<link rel=\"modulepreload\" href=\"/static/js/offline/mutations.js\"></head><body>";
  const stub = "<script>window.PWAMS_PERMISSIONS={};window.PWAMS_I18N={};"
    + "window.t=(k)=>({\"offline.session_expired\":\"Your offline session has expired. Reconnect to the server and sign in again.\","
    + "\"offline.requires_online\":\"This feature requires an internet connection.\","
    + "\"offline.available\":\"Offline data available.\"}[k]??k);</script>";
  const tail = "<main><div data-sync-conflicts hidden></div>" + mainHtml + "</main>"
    + "<div class=\"offline-status\" data-offline-status data-state=\"online\" role=\"status\" aria-live=\"polite\">Online</div>"
    + "<script type=\"module\" src=\"/static/js/offline/register.js\"></script>"
    + "<script type=\"module\" src=\"/static/js/offline/pages.js\"></script>"
    + "</body></html>";
  return head + stub + tail;
}

export const personsPage = pageShell("Persons",
  "<h1>Persons</h1><table class=\"data-table\"><thead><tr><th>Full Name</th><th>NIC / Passport</th><th>Phone</th><th>Status</th><th>Actions</th></tr></thead><tbody></tbody></table>");

export const reportsPage = pageShell("Reports", "<h1>Reports</h1><p>Report content</p>");

export const seedPage = "<!DOCTYPE html><html><head><meta charset=\"UTF-8\"></head><body><script type=\"module\">"
  + "const names=['persons','students','donors','donations','aid_requests','loans','loan_repayments','care_provided','revenue','outbox','metadata'];"
  + "const req=indexedDB.open('pwams-offline',4);"
  + "req.onupgradeneeded=()=>{const db=req.result;for(const s of names){if(!db.objectStoreNames.contains(s)){const os=db.createObjectStore(s,{keyPath:'id',autoIncrement:s==='outbox'});if(s!=='outbox'&&s!=='metadata')os.createIndex('updatedAt','updatedAt',{unique:false});}}};"
  + "req.onsuccess=()=>{const db=req.result;const tx=db.transaction('persons','readwrite');"
  + "tx.objectStore('persons').put({id:'p1',full_name:'Alice Seeker',nic_passport:'952000000V',phone:'0700000000',status:'active',updatedAt:new Date().toISOString()});"
  + "tx.oncomplete=()=>{const tx2=db.transaction('metadata','readwrite');tx2.objectStore('metadata').put({id:'offline_session',lastAuthenticatedAt:Date.now()});"
  + "tx2.objectStore('metadata').put({id:'offline_user_id',value:'admin'});tx2.oncomplete=()=>{document.body.dataset.ready='1';};};};"
  + "</script></body></html>";