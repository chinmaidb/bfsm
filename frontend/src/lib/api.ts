export type Session = {
  accessToken: string
  userId: string
  username: string
  role: 'admin' | 'staff' | 'promoter' | 'member'
  permissions: string[]
  isSuperAdmin: boolean
}

const API_BASE = (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/$/, '')
const SESSION_KEY = 'schemeapp.session'

export function getSession(): Session | null {
  try {
    const value = localStorage.getItem(SESSION_KEY)
    return value ? JSON.parse(value) as Session : null
  } catch {
    return null
  }
}

export function saveSession(session: Session | null) {
  if (session) localStorage.setItem(SESSION_KEY, JSON.stringify(session))
  else localStorage.removeItem(SESSION_KEY)
}

export function hasPermission(permission: string): boolean {
  const session = getSession()
  return Boolean(session?.isSuperAdmin || session?.permissions?.includes(permission))
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const session = getSession()
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (session?.accessToken) headers.set('Authorization', `Bearer ${session.accessToken}`)
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers })
  if (response.status === 401) {
    saveSession(null)
    window.dispatchEvent(new Event('schemeapp:unauthorized'))
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { message?: string }
    throw new Error(body.message || `Request failed (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export async function login(username: string, password: string): Promise<Session> {
  const result = await api<{ userId: string; username: string; role: Session['role']; permissions: string[]; isSuperAdmin: boolean; accessToken: string }>(
    '/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) },
  )
  const session: Session = result
  saveSession(session)
  return session
}
