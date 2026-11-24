/**
 * A UUIDv7: 48 bits of Unix milliseconds, then random bits (RFC 9562).
 * Client-generated ids make every write idempotent (SYSTEM_DESIGN §1.1):
 * a retried request carries the same id, so the server records it once.
 */
export function uuidv7(now: number = Date.now()): string {
  const b = new Uint8Array(16)
  crypto.getRandomValues(b)
  b[0] = Math.floor(now / 2 ** 40) & 0xff
  b[1] = Math.floor(now / 2 ** 32) & 0xff
  b[2] = (now >>> 24) & 0xff
  b[3] = (now >>> 16) & 0xff
  b[4] = (now >>> 8) & 0xff
  b[5] = now & 0xff
  b[6] = 0x70 | (b[6] & 0x0f) // version 7
  b[8] = 0x80 | (b[8] & 0x3f) // variant 10
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

const DEVICE_KEY = 'fuxi.device'

/** This browser's device id, created once and kept (PUT /v1/devices/{id}). */
export function deviceId(): string {
  try {
    let id = localStorage.getItem(DEVICE_KEY)
    if (!id) {
      id = uuidv7()
      localStorage.setItem(DEVICE_KEY, id)
    }
    return id
  } catch {
    // Storage blocked (private mode): a per-session device still works online.
    return (sessionDevice ??= uuidv7())
  }
}
let sessionDevice: string | undefined

/** A readable device name for the account's device list, e.g. "iPhone · Safari". */
export function deviceName(ua: string = navigator.userAgent): string {
  const os = /iPhone/.test(ua)
    ? 'iPhone'
    : /iPad/.test(ua)
      ? 'iPad'
      : /Android/.test(ua)
        ? 'Android'
        : /Windows/.test(ua)
          ? 'Windows'
          : /Mac OS X/.test(ua)
            ? 'Mac'
            : /Linux/.test(ua)
              ? 'Linux'
              : 'Device'
  const browser = /Edg\//.test(ua)
    ? 'Edge'
    : /Firefox\//.test(ua)
      ? 'Firefox'
      : /CriOS|Chrome\//.test(ua)
        ? 'Chrome'
        : /Safari\//.test(ua)
          ? 'Safari'
          : 'Browser'
  return `${os} · ${browser}`
}
