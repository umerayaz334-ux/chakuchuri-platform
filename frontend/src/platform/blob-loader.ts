// Share only pending downloads. Completed private files remain governed by HTTP caching.
export function createBlobLoader(fetcher: typeof fetch = fetch) {
  const pending = new Map<string, Promise<Blob>>();
  return (path: string, token?: string): Promise<Blob> => {
    const key = JSON.stringify([token || "", path]);
    const existing = pending.get(key);
    if (existing) return existing;
    const request = (async () => {
      const response = await fetcher(path, {
        cache: "no-cache",
        headers: token ? { Authorization: `Bearer ${token}` } : undefined
      });
      if (!response.ok) throw new Error("File could not be loaded.");
      return response.blob();
    })();
    if (pending.size >= 64) return request;
    const tracked = request.finally(() => { pending.delete(key); });
    pending.set(key, tracked);
    return tracked;
  };
}
