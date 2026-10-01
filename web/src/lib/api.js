export class Unauthorized extends Error {}

async function request(path, opts = {}) {
  const r = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    ...opts,
  })
  if (r.status === 401) throw new Unauthorized('unauthorized')
  if (!r.ok) throw new Error((await r.text()) || `HTTP ${r.status}`)
  const ct = r.headers.get('content-type') || ''
  if (!ct.includes('application/json')) return null
  return r.json()
}

function qs(params = {}) {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') p.set(k, v)
  }
  const s = p.toString()
  return s ? `?${s}` : ''
}

export const api = {
  status: () => request('/api/auth/status'),
  login: (password) => request('/api/auth/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request('/api/auth/logout', { method: 'POST' }),

  notes: (params) => request('/api/notes' + qs(params)),
  recent: () => request('/api/recent'),
  note: (id) => request(`/api/notes/${id}`),
  createNote: (note) => request('/api/notes', { method: 'POST', body: JSON.stringify(note) }),
  updateNote: (id, patch) => request(`/api/notes/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  deleteNote: (id) => request(`/api/notes/${id}`, { method: 'DELETE' }),

  folders: () => request('/api/folders'),
  createFolder: (name) => request('/api/folders', { method: 'POST', body: JSON.stringify({ name }) }),
  renameFolder: (name, next) => request(`/api/folders/${encodeURIComponent(name)}`, { method: 'PATCH', body: JSON.stringify({ name: next }) }),
  deleteFolder: (name) => request(`/api/folders/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  search: (q, params) => request('/api/search' + qs({ q, ...params })),
  capture: (raw) => request('/api/capture', { method: 'POST', body: JSON.stringify({ raw }) }),
  agent: (payload) => request('/api/agent', { method: 'POST', body: JSON.stringify(payload) }),
  settings: () => request('/api/settings'),
  saveSettings: (s) => request('/api/settings', { method: 'PUT', body: JSON.stringify(s) }),
}
