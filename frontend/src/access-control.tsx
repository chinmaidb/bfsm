import { useEffect, useState, type FormEvent } from 'react'
import { KeyRound, Plus, RefreshCw, ShieldCheck, UserCog, X } from 'lucide-react'
import { api, getSession } from './lib/api'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'

type Account = { id: string; username: string; role: string; roleId: string; roleName: string; active: boolean; isSuperAdmin: boolean; createdAt: string }
type CompanyRole = { id: string; roleKey: string; name: string; description: string; permissions: string[]; active: boolean }
type Permission = { key: string; label: string }

const capabilities = [
  ['dashboard.read', 'View dashboard'],
  ['promoters.read', 'View promoters'], ['promoters.create', 'Create promoters'], ['promoters.update', 'Edit promoters'], ['promoters.delete', 'Deactivate promoters'],
  ['members.read', 'View members'], ['members.create', 'Create members'], ['members.update', 'Edit members'], ['members.delete', 'Deactivate members'],
  ['schemes.read', 'View schemes'], ['schemes.create', 'Create schemes'], ['schemes.update', 'Edit schemes'], ['schemes.delete', 'Deactivate schemes'],
  ['enrollments.read', 'View enrollments'], ['enrollments.create', 'Create enrollments'], ['enrollments.update', 'Change enrollment status'],
  ['payments.read', 'View payments and receipts'], ['payments.create', 'Record payments'],
  ['draws.read', 'View draw results'], ['draws.create', 'Record chosen members'],
  ['incentives.read', 'View incentives'], ['incentives.create', 'Create incentives'], ['incentives.update', 'Update incentive status'],
  ['parties.read', 'View customers and suppliers'], ['parties.create', 'Create customers and suppliers'], ['parties.update', 'Edit customers and suppliers'], ['parties.delete', 'Deactivate customers and suppliers'],
  ['cash_receipts.read', 'View customer receipts'], ['cash_receipts.create', 'Record customer receipts'],
  ['supplier_payments.read', 'View supplier payments'], ['supplier_payments.create', 'Record supplier payments'],
  ['reports.read', 'View reports'], ['audit.read', 'View audit trail'],
  ['accounts.read', 'View accounts'], ['accounts.create', 'Create accounts'], ['accounts.update', 'Edit accounts'], ['accounts.delete', 'Deactivate accounts'],
  ['roles.read', 'View roles'], ['roles.create', 'Create roles'], ['roles.update', 'Edit roles'], ['roles.delete', 'Deactivate roles'],
] as const

export function AccessControlPage({ view }: { view: 'accounts' | 'roles' }) {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [roles, setRoles] = useState<CompanyRole[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [form, setForm] = useState<Account | CompanyRole | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const session = getSession()
  const grants = new Set(session?.permissions || [])
  const isSuperAdmin = Boolean(session?.isSuperAdmin)
  const can = (permission: string) => isSuperAdmin || grants.has(permission)

  async function load() {
    setLoading(true); setError('')
    try {
      if (view === 'accounts') {
        const requests: Promise<unknown>[] = [api<Account[]>('/accounts')]
        if (can('roles.read')) requests.push(api<CompanyRole[]>('/roles'))
        const [a, r] = await Promise.all(requests)
        setAccounts(a as Account[])
        if (r) setRoles(r as CompanyRole[])
      } else {
        const [r, p] = await Promise.all([api<CompanyRole[]>('/roles'), api<Permission[]>('/permissions')])
        setRoles(r); setPermissions(p)
      }
    } catch (err) { setError(err instanceof Error ? err.message : 'Could not load access settings') }
    finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [view])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError('')
    const values = new FormData(event.currentTarget)
    try {
      if (view === 'accounts') {
        const editing = form as Account | null
        const body: Record<string, unknown> = { username: String(values.get('username')), roleId: String(values.get('roleId')), active: values.get('active') === 'on' }
        if (values.get('password')) body.password = String(values.get('password'))
        const saved = await api<Account>(editing?.id ? `/accounts/${editing.id}` : '/accounts', { method: editing?.id ? 'PUT' : 'POST', body: JSON.stringify(body) })
        setAccounts(current => editing?.id ? current.map(item => item.id === saved.id ? saved : item) : [saved, ...current])
      } else {
        const editing = form as CompanyRole | null
        const body = { name: String(values.get('name')), description: String(values.get('description') || ''), permissions: values.getAll('permissions').map(String) }
        const saved = await api<CompanyRole>(editing?.id ? `/roles/${editing.id}` : '/roles', { method: editing?.id ? 'PUT' : 'POST', body: JSON.stringify(body) })
        setRoles(current => editing?.id ? current.map(item => item.id === saved.id ? saved : item) : [...current, saved].sort((a,b) => a.name.localeCompare(b.name)))
      }
      window.dispatchEvent(new Event('schemeapp:access-changed'))
      setForm(null)
    } catch (err) { setError(err instanceof Error ? err.message : 'Could not save changes') }
    finally { setSaving(false) }
  }

  async function deactivate(item: Account | CompanyRole) {
    const label = 'username' in item ? item.username : item.name
    if (!window.confirm(`Deactivate ${label}? This change will be recorded in the audit trail.`)) return
    setError('')
    try {
      await api('username' in item ? `/accounts/${item.id}` : `/roles/${item.id}`, { method: 'DELETE' })
      if ('username' in item) setAccounts(current => current.map(account => account.id === item.id ? { ...account, active: false } : account))
      else setRoles(current => current.map(role => role.id === item.id ? { ...role, active: false } : role))
      window.dispatchEvent(new Event('schemeapp:access-changed'))
    } catch (err) { setError(err instanceof Error ? err.message : 'Could not deactivate') }
  }

  const isAccounts = view === 'accounts'
  const title = isAccounts ? 'Company accounts' : 'Roles & permissions'
  const mayCreate = can(isAccounts ? 'accounts.create' : 'roles.create')
  const mayEdit = can(isAccounts ? 'accounts.update' : 'roles.update')
  const mayDelete = can(isAccounts ? 'accounts.delete' : 'roles.delete')
  const current = form as Account | CompanyRole | null
  const editing = Boolean(current?.id)

  return <div className="page-enter space-y-5">
    <div className="flex flex-wrap items-end justify-between gap-3"><div><p className="text-[10px] font-bold uppercase tracking-[.18em] text-indigo-600">Access control</p><h1 className="mt-1 font-display text-2xl font-extrabold tracking-tight text-slate-950">{title}</h1><p className="mt-1 max-w-2xl text-sm text-slate-500">{isAccounts ? 'Manage company operator and administrator logins. The configured super admin remains protected.' : 'Define which operations each company role can perform. Permission checks are enforced by the API.'}</p></div><div className="flex gap-2"><Button variant="outline" onClick={() => void load()}><RefreshCw size={14}/>Refresh</Button>{mayCreate && (isAccounts ? can('roles.read') : true) && <Button onClick={() => setForm(isAccounts ? {id:'',username:'',role:'staff',roleId:'',roleName:'',active:true,isSuperAdmin:false,createdAt:''} : {id:'',roleKey:'',name:'',description:'',permissions:[],active:true})}><Plus size={15}/>{isAccounts ? 'Add account' : 'Create role'}</Button>}</div></div>
    {error && <div role="alert" className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</div>}
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm"><div className="border-b border-slate-100 px-5 py-4"><div className="flex items-center gap-2 text-sm font-bold text-slate-800">{isAccounts ? <UserCog size={16}/> : <KeyRound size={16}/>} {isAccounts ? 'Accounts' : 'Company roles'}</div></div>
      {loading ? <div className="p-8 text-center text-sm text-slate-400">Loading access settings…</div> : isAccounts ? <div className="overflow-x-auto"><table className="w-full min-w-[680px] text-left text-sm"><thead className="bg-slate-50 text-[10px] uppercase tracking-wider text-slate-400"><tr><th className="px-5 py-3">Account</th><th className="px-5 py-3">Role</th><th className="px-5 py-3">Status</th><th className="px-5 py-3 text-right">Actions</th></tr></thead><tbody className="divide-y divide-slate-100">{accounts.map(account => <tr key={account.id}><td className="px-5 py-4 font-semibold text-slate-800">{account.username}{account.isSuperAdmin&&<span className="ml-2 rounded-full bg-violet-50 px-2 py-1 text-[9px] font-bold uppercase text-violet-700">Config super admin</span>}</td><td className="px-5 py-4 text-slate-500">{account.roleName || account.role}</td><td className="px-5 py-4"><span className={`rounded-full px-2.5 py-1 text-[10px] font-bold ${account.active?'bg-emerald-50 text-emerald-700':'bg-slate-100 text-slate-500'}`}>{account.active?'Active':'Inactive'}</span></td><td className="px-5 py-4"><div className="flex justify-end gap-1">{mayEdit&&!account.isSuperAdmin&&['staff','admin'].includes(account.role)&&<Button variant="ghost" size="sm" onClick={()=>setForm(account)}>Edit</Button>}{mayDelete&&!account.isSuperAdmin&&['staff','admin'].includes(account.role)&&account.active&&account.id!==session?.userId&&<Button variant="ghost" size="sm" className="text-rose-600" onClick={()=>void deactivate(account)}>Deactivate</Button>}</div></td></tr>)}</tbody></table>{accounts.length===0&&<p className="p-8 text-center text-sm text-slate-400">No company accounts yet.</p>}</div> : <div className="divide-y divide-slate-100">{roles.map(role=><div key={role.id} className="flex flex-wrap items-start justify-between gap-3 px-5 py-4"><div className="min-w-0"><div className="flex items-center gap-2"><p className="text-sm font-bold text-slate-800">{role.name}</p>{role.roleKey==='company_admin'&&<ShieldCheck size={14} className="text-indigo-600"/>}<span className={`rounded-full px-2 py-0.5 text-[9px] font-bold ${role.active?'bg-emerald-50 text-emerald-700':'bg-slate-100 text-slate-500'}`}>{role.active?'Active':'Inactive'}</span></div><p className="mt-1 text-xs text-slate-500">{role.description || 'No description'} · {role.permissions.length} permissions</p><p className="mt-2 line-clamp-2 text-[11px] text-slate-400">{role.permissions.map(key=>capabilities.find(pair=>pair[0]===key)?.[1]||key).join(' · ') || 'No operations granted'}</p></div><div className="flex gap-1">{mayEdit&&role.roleKey!=='company_admin'&&<Button variant="ghost" size="sm" onClick={()=>setForm(role)}>Edit</Button>}{mayDelete&&role.roleKey!=='company_admin'&&role.active&&<Button variant="ghost" size="sm" className="text-rose-600" onClick={()=>void deactivate(role)}>Deactivate</Button>}</div></div>)}{roles.length===0&&<p className="p-8 text-center text-sm text-slate-400">No company roles are available.</p>}</div>}
    </section>

    {current && <div className="fixed inset-0 z-50 flex items-end justify-center bg-slate-950/40 p-0 backdrop-blur-[2px] sm:items-center sm:p-6"><section role="dialog" aria-modal="true" className="max-h-[92vh] w-full overflow-y-auto rounded-t-3xl bg-white p-6 shadow-2xl sm:max-w-2xl sm:rounded-3xl"><div className="mb-5 flex items-center justify-between"><div><p className="text-[10px] font-bold uppercase tracking-[.16em] text-indigo-600">Access control</p><h2 className="mt-1 text-xl font-bold">{isAccounts ? (editing ? 'Edit account' : 'Create account') : (editing ? 'Edit role' : 'Create role')}</h2></div><Button variant="ghost" size="icon" onClick={()=>setForm(null)} aria-label="Close"><X size={18}/></Button></div><form onSubmit={submit} className="space-y-4">
      {isAccounts ? <><label className="block text-xs font-semibold text-slate-600">Username<Input name="username" minLength={3} maxLength={80} required defaultValue={'username' in current ? current.username : ''} className="mt-1.5"/></label><label className="block text-xs font-semibold text-slate-600">Company role<select name="roleId" required defaultValue={'roleId' in current ? current.roleId : ''} className="mt-1.5 h-10 w-full rounded-xl border border-slate-200 bg-white px-3 text-sm"><option value="" disabled>Choose a role</option>{roles.filter(role=>role.active).map(role=><option key={role.id} value={role.id}>{role.name}</option>)}</select></label><label className="block text-xs font-semibold text-slate-600">{editing ? 'New password (leave empty to keep current)' : 'Initial password (12+ characters)'}<Input name="password" type="password" minLength={12} required={!editing} autoComplete="new-password" className="mt-1.5"/></label>{editing&&'username' in current&&<label className="flex items-center gap-2 text-xs font-semibold text-slate-600"><input name="active" type="checkbox" defaultChecked={current.active} className="size-4 rounded border-slate-300"/>Account can sign in</label>}</> : <><label className="block text-xs font-semibold text-slate-600">Role name<Input name="name" required minLength={2} maxLength={80} defaultValue={'name' in current?current.name:''} className="mt-1.5"/></label><label className="block text-xs font-semibold text-slate-600">Description<Input name="description" maxLength={500} defaultValue={'description' in current?current.description:''} className="mt-1.5"/></label><fieldset><legend className="mb-2 text-xs font-bold text-slate-700">Allowed operations</legend><div className="grid gap-2 sm:grid-cols-2">{(permissions.length?permissions:capabilities.map(([key,label])=>({key,label}))).map(permission=><label key={permission.key} className="flex items-start gap-2 rounded-xl border border-slate-100 px-3 py-2 text-xs text-slate-600"><input name="permissions" type="checkbox" value={permission.key} defaultChecked={'permissions' in current&&current.permissions.includes(permission.key)} className="mt-0.5 size-3.5 rounded border-slate-300"/><span>{permission.label}</span></label>)}</div></fieldset></>}
      <div className="flex justify-end gap-2 border-t border-slate-100 pt-4"><Button type="button" variant="outline" onClick={()=>setForm(null)}>Cancel</Button><Button disabled={saving}>{saving?'Saving…':'Save changes'}</Button></div></form></section></div>}
  </div>
}
