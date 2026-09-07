import { useEffect, useState } from "react";

/** Shared clock for online / last-seen labels so every screen ages the same way. */
export function usePresenceNow(intervalMs = 5_000) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(timer);
  }, [intervalMs]);
  return now;
}
