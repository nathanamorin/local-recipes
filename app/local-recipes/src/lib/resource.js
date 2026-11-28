// A tiny stale-while-revalidate cache. Pages read through useResource so
// revisiting a page renders cached data instantly while a fresh copy loads
// in the background; mutations call invalidate()/setCached() so every view
// of the same data stays consistent without a global store.
import { useCallback, useEffect, useState, useSyncExternalStore } from 'react'

const cache = new Map() // key -> { data, error, promise, at }
const listeners = new Set()
let version = 0

function emit() {
  version++
  listeners.forEach((l) => l())
}

function subscribe(listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function load(key, fetcher) {
  const entry = cache.get(key) || {}
  if (entry.promise) return entry.promise
  const promise = fetcher()
    .then((data) => {
      cache.set(key, { data, error: null, promise: null, at: Date.now() })
      emit()
      return data
    })
    .catch((error) => {
      cache.set(key, { ...cache.get(key), error, promise: null })
      emit()
      throw error
    })
  cache.set(key, { ...entry, promise })
  return promise
}

export function setCached(key, updater) {
  const entry = cache.get(key)
  const data = typeof updater === 'function' ? updater(entry ? entry.data : undefined) : updater
  cache.set(key, { ...(entry || {}), data, error: null, at: Date.now() })
  emit()
}

export function getCached(key) {
  const entry = cache.get(key)
  return entry ? entry.data : undefined
}

// invalidate drops every entry whose key starts with prefix and refetches
// any that are currently mounted (they refetch on the next render).
export function invalidate(prefix) {
  for (const key of cache.keys()) {
    if (key.startsWith(prefix)) {
      const entry = cache.get(key)
      cache.set(key, { ...entry, at: 0 })
    }
  }
  emit()
}

const STALE_MS = 15_000

export function useResource(key, fetcher) {
  useSyncExternalStore(subscribe, () => version)
  const entry = key ? cache.get(key) : undefined
  const [, force] = useState(0)

  useEffect(() => {
    if (!key) return
    const e = cache.get(key)
    if (!e || (!e.promise && Date.now() - (e.at || 0) > STALE_MS)) {
      load(key, fetcher).catch(() => {})
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, entry && entry.at])

  const reload = useCallback(() => {
    if (!key) return Promise.resolve()
    const e = cache.get(key)
    if (e) cache.set(key, { ...e, at: 0, error: null })
    force((n) => n + 1)
    return load(key, fetcher)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  return {
    data: entry ? entry.data : undefined,
    error: entry && entry.data === undefined ? entry.error : null,
    loading: !entry || (entry.data === undefined && !entry.error),
    refreshing: !!(entry && entry.promise && entry.data !== undefined),
    reload,
  }
}
