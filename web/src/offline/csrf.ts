/**
 * Reads the double-submit CSRF token issued by the server
 * (pwams_csrf cookie). Used by all client code that performs unsafe
 * (POST/PUT/PATCH/DELETE) fetch requests.
 */
export function getCsrfToken(): string {
  const match = document.cookie.match(/(^| )pwams_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[2]) : "";
}
