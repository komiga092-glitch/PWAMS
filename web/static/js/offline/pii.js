const PII_FIELDS = [
    "full_name",
    "nic_passport",
    "phone",
    "email",
    "address",
    "date_of_birth",
    "occupation",
    "monthly_income",
];
const ALGORITHM = "AES-GCM";
const KEY_LENGTH = 256;
const IV_LENGTH = 12;
let cryptoKey = null;
async function getCryptoKey() {
    if (cryptoKey)
        return cryptoKey;
    const stored = localStorage.getItem("pwams_pii_key");
    if (stored) {
        const raw = Uint8Array.from(atob(stored), (c) => c.charCodeAt(0));
        cryptoKey = await crypto.subtle.importKey("raw", raw, { name: ALGORITHM, length: KEY_LENGTH }, false, ["encrypt", "decrypt"]);
        return cryptoKey;
    }
    cryptoKey = await crypto.subtle.generateKey({ name: ALGORITHM, length: KEY_LENGTH }, true, ["encrypt", "decrypt"]);
    const exported = await crypto.subtle.exportKey("raw", cryptoKey);
    localStorage.setItem("pwams_pii_key", btoa(String.fromCharCode(...new Uint8Array(exported))));
    return cryptoKey;
}
export async function encryptPII(value) {
    if (!value)
        return value;
    const key = await getCryptoKey();
    const iv = crypto.getRandomValues(new Uint8Array(IV_LENGTH));
    const encoder = new TextEncoder();
    const encoded = encoder.encode(value);
    const encrypted = await crypto.subtle.encrypt({ name: ALGORITHM, iv }, key, encoded);
    const combined = new Uint8Array(iv.length + encrypted.byteLength);
    combined.set(iv, 0);
    combined.set(new Uint8Array(encrypted), iv.length);
    return btoa(String.fromCharCode(...combined));
}
export async function decryptPII(value) {
    if (!value)
        return value;
    try {
        const key = await getCryptoKey();
        const combined = Uint8Array.from(atob(value), (c) => c.charCodeAt(0));
        const iv = combined.slice(0, IV_LENGTH);
        const data = combined.slice(IV_LENGTH);
        const decrypted = await crypto.subtle.decrypt({ name: ALGORITHM, iv }, key, data);
        return new TextDecoder().decode(decrypted);
    }
    catch {
        return value;
    }
}
export async function encryptPIIFields(record) {
    const result = { ...record };
    for (const field of PII_FIELDS) {
        if (field in result && typeof result[field] === "string") {
            result[field] = await encryptPII(result[field]);
        }
    }
    return result;
}
export async function decryptPIIFields(record) {
    const result = { ...record };
    for (const field of PII_FIELDS) {
        if (field in result && typeof result[field] === "string") {
            result[field] = await decryptPII(result[field]);
        }
    }
    return result;
}
//# sourceMappingURL=pii.js.map