import { encryptField, decryptField, PII_FIELDS } from "./pii";
describe("PII encryption", () => {
    const testKey = crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, true, ["encrypt", "decrypt"]);
    it("encrypts and decrypts round trip", async () => {
        const key = await testKey;
        const plaintext = "sensitive-data-123";
        const encrypted = await encryptField(plaintext, key);
        expect(encrypted).not.toBe(plaintext);
        const decrypted = await decryptField(encrypted, key);
        expect(decrypted).toBe(plaintext);
    });
    it("has correct PII fields list", () => {
        expect(PII_FIELDS).toContain("nic_passport");
        expect(PII_FIELDS).toContain("phone");
        expect(PII_FIELDS).toContain("email");
        expect(PII_FIELDS).toContain("address");
    });
    it("decrypt returns empty string for invalid data", async () => {
        const key = await testKey;
        const result = await decryptField("not-valid-base64!", key);
        expect(result).toBe("");
    });
});
//# sourceMappingURL=pii_test.js.map