export async function readOfflineStore(store) {
    const database = await import(String("/static/js/offline/db.js"));
    const listOfflineRecords = database.listOfflineRecords;
    return listOfflineRecords(store);
}
//# sourceMappingURL=offline-data.js.map