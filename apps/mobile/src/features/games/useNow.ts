import { useEffect, useState } from 'react';

// Current time, refreshed every `ms` while mounted; for countdowns.
export function useNow(ms = 500) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(id);
  }, [ms]);
  return now;
}
