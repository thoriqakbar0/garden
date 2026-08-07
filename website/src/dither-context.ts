import { useSyncExternalStore } from "react";

let ditherScale = 1;
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot() {
  return ditherScale;
}

export function setDitherScale(value: number) {
  if (value === ditherScale) return;
  ditherScale = value;
  listeners.forEach((listener) => listener());
}

export function useDitherScale() {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
