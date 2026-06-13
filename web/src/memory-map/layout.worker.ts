// Web Worker: offloads force-directed layout computation off the main thread.
// Accepts { nodes, relations } and posts back { positions }.
// This keeps the UI responsive while the O(n²) force simulation runs.

import { computeLayout } from './layout';

self.onmessage = (e: MessageEvent<{ nodes: Parameters<typeof computeLayout>[0]; relations: Parameters<typeof computeLayout>[1] }>) => {
  const { nodes, relations } = e.data;
  const result = computeLayout(nodes, relations);
  self.postMessage(result);
};
