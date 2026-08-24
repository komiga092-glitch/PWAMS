export type ConnectivityListener = (online: boolean) => void;

let online = navigator.onLine;
const listeners = new Set<ConnectivityListener>();

function publish(nextOnline: boolean): void {
  if (online === nextOnline) return;
  online = nextOnline;
  listeners.forEach((listener) => listener(online));
}

window.addEventListener("online", () => publish(true));
window.addEventListener("offline", () => publish(false));

export function isOnline(): boolean {
  return online;
}

export function subscribeConnectivity(
  listener: ConnectivityListener,
): () => void {
  listeners.add(listener);
  listener(online);
  return () => listeners.delete(listener);
}
