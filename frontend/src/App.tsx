import { useEffect, useState, type FormEvent } from 'react'
import {
  Activity, ArrowDownToLine, BriefcaseBusiness, Building2, ChartNoAxesCombined,
  CircleDollarSign, ClipboardList, FileClock, Gift, LayoutDashboard, LogOut, Menu, ShieldCheck,
  Sparkles, TicketCheck, Users, WalletCards, Store, Receipt, HandCoins, UserCog, KeyRound,
} from 'lucide-react'
import { api, getSession, login, saveSession, type Session } from './lib/api'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { AccessControlPage } from './access-control'
import {
  AuditPage, Dashboard, DataTable, DrawPage, EnrollmentPage, IncentivePage, MemberProfilePage,
  PageHeading, PartyPage, PaymentPage, FinancialLedgerPage, ReceiptPage, ReportsPage, ResourcePage, type Row,
} from './screens'

type Page = 'overview' | 'members' | 'promoters' | 'schemes' | 'enrollments' | 'payments' | 'draws' | 'incentives' | 'reports' | 'audit' | 'profile' | 'customers' | 'suppliers' | 'customerReceipts' | 'supplierPayments' | 'accounts' | 'roles'
const icons: Record<Page, typeof LayoutDashboard> = { overview: LayoutDashboard, members: Users, promoters: BriefcaseBusiness, schemes: Building2, enrollments: ClipboardList, payments: WalletCards, draws: TicketCheck, incentives: Gift, reports: ChartNoAxesCombined, audit: FileClock, profile: ShieldCheck, customers: Users, suppliers: Store, customerReceipts: Receipt, supplierPayments: HandCoins, accounts: UserCog, roles: KeyRound }
const adminNav: { id: Page; label: string; section: string }[] = [
    { id: 'overview', label: 'Overview', section: 'Workspace' }, { id: 'members', label: 'Members', section: 'Workspace' }, { id: 'promoters', label: 'Promoters', section: 'Workspace' },
    { id: 'schemes', label: 'Schemes', section: 'Scheme operations' }, { id: 'enrollments', label: 'Enrollments', section: 'Scheme operations' }, { id: 'payments', label: 'Member contributions', section: 'Scheme operations' }, { id: 'draws', label: 'Draw results', section: 'Scheme operations' }, { id: 'incentives', label: 'Incentives', section: 'Scheme operations' },
    { id: 'customers', label: 'Customers', section: 'Cash ledger' }, { id: 'suppliers', label: 'Suppliers', section: 'Cash ledger' }, { id: 'customerReceipts', label: 'Customer receipts', section: 'Cash ledger' }, { id: 'supplierPayments', label: 'Supplier payments', section: 'Cash ledger' },
    { id: 'reports', label: 'Reports', section: 'Insights' }, { id: 'audit', label: 'Audit trail', section: 'Insights' },
    { id: 'accounts', label: 'Accounts', section: 'Access control' }, { id: 'roles', label: 'Roles & permissions', section: 'Access control' },
  ]
const nav: Record<Session['role'], { id: Page; label: string; section: string }[]> = {
  admin: adminNav,
  staff: adminNav,
  promoter: [
    { id: 'overview', label: 'Overview', section: 'Workspace' }, { id: 'members', label: 'My members', section: 'Workspace' }, { id: 'enrollments', label: 'Enrollments', section: 'Operations' }, { id: 'payments', label: 'Payments & receipts', section: 'Operations' }, { id: 'incentives', label: 'My incentives', section: 'Earnings' },
  ],
  member: [
    { id: 'overview', label: 'Overview', section: 'My scheme' }, { id: 'profile', label: 'My profile', section: 'My scheme' }, { id: 'enrollments', label: 'My enrollments', section: 'My scheme' }, { id: 'payments', label: 'Payments & receipts', section: 'My scheme' },
  ],
}
const pagePermissions: Partial<Record<Page, string[]>> = {
  overview: ['dashboard.read', 'payments.read', 'enrollments.read', 'members.read', 'promoters.read'], members: ['members.read'], promoters: ['promoters.read'], schemes: ['schemes.read'],
  enrollments: ['enrollments.read'], payments: ['payments.read'], draws: ['draws.read', 'schemes.read', 'members.read'], incentives: ['incentives.read'],
  customers: ['parties.read'], suppliers: ['parties.read'], customerReceipts: ['cash_receipts.read', 'parties.read'],
  supplierPayments: ['supplier_payments.read', 'parties.read'], reports: ['reports.read'], audit: ['audit.read'],
  accounts: ['accounts.read'], roles: ['roles.read'],
}

export function App() {
  const [session, setSession] = useState<Session | null>(() => getSession())
  const [page, setPage] = useState<Page>('overview')
  const [menuOpen, setMenuOpen] = useState(false)
  useEffect(() => {
    const clear = () => setSession(null)
    window.addEventListener('schemeapp:unauthorized', clear)
    return () => window.removeEventListener('schemeapp:unauthorized', clear)
  }, [])
  useEffect(() => {
    if (!session) return
    const refreshAccess = () => { void api<{ permissions: string[]; isSuperAdmin: boolean }>('/auth/me').then(access => {
      setSession(current => {
        if (!current) return current
        const next = { ...current, ...access }
        saveSession(next)
        return next
      })
    }).catch(() => undefined) }
    void refreshAccess()
    window.addEventListener('schemeapp:access-changed', refreshAccess)
    return () => window.removeEventListener('schemeapp:access-changed', refreshAccess)
  }, [session?.userId])
  async function onLogin(username: string, password: string) { const value = await login(username, password);setSession(value);setPage('overview') }
  function onLogout() { saveSession(null);setSession(null);setPage('overview') }
  if (!session) return <Login onLogin={onLogin} />
  if (window.location.pathname === '/receipt') return <ReceiptPage role={session.role}/>
  const baseItems = nav[session.role]
  const items = session.role === 'admin' || session.role === 'staff'
    ? baseItems.filter(item => session.isSuperAdmin || (pagePermissions[item.id] || []).every(permission => session.permissions?.includes(permission)))
    : baseItems
  const current = items.find(item => item.id === page) || items[0] || { id: 'overview' as Page, label: 'No access', section: '' }
  const Icon = icons[current.id]
  const set = (next: Page) => { setPage(next);setMenuOpen(false) }
  return <div className="min-h-screen bg-[#f7f8fb] text-slate-900">
    <aside className="fixed inset-y-0 left-0 z-30 hidden w-62.5 flex-col bg-[#11182b] text-white lg:flex">
      <Brand />
      <Nav items={items} active={page} onSelect={set} />
      <div className="mt-auto px-4 pb-5"><div className="rounded-2xl border border-white/10 bg-white/4.5 p-4"><div className="flex size-8 items-center justify-center rounded-xl bg-indigo-500/20 text-indigo-300"><Sparkles size={16}/></div><p className="mt-3 text-xs font-semibold text-white">A scheme ledger you can trust.</p><p className="mt-1 text-[11px] leading-relaxed text-slate-400">Every contribution and change leaves a clear trail.</p></div><Account session={session} onLogout={onLogout}/></div>
    </aside>
    {menuOpen && <div className="fixed inset-0 z-40 bg-slate-950/40 lg:hidden" onClick={() => setMenuOpen(false)}><aside onClick={e => e.stopPropagation()} className="flex h-full w-70 flex-col bg-[#11182b] text-white"><Brand/><Nav items={items} active={page} onSelect={set}/><div className="mt-auto px-4 pb-5"><Account session={session} onLogout={onLogout}/></div></aside></div>}
    <div className="min-h-screen lg:pl-62.5">
      <header className="sticky top-0 z-20 flex h-18 items-center justify-between border-b border-slate-200/80 bg-white/90 px-4 backdrop-blur-xl sm:px-7">
        <div className="flex min-w-0 items-center gap-3"><Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setMenuOpen(true)} aria-label="Open navigation"><Menu size={19}/></Button><div className="hidden text-xs text-slate-400 sm:block">Benefit scheme <span className="mx-2 text-slate-300">/</span></div><div className="flex min-w-0 items-center gap-2"><span className="text-indigo-600"><Icon size={16}/></span><span className="truncate text-sm font-semibold text-slate-800">{current.label}</span></div></div>
        <div className="flex items-center gap-3 sm:gap-4"><div className="hidden h-7 w-px bg-slate-200 sm:block"/><div className="hidden text-right sm:block"><p className="text-xs font-bold text-slate-800">{session.username}</p><p className="text-[10px] text-slate-400">{session.isSuperAdmin?'Super admin · configuration bypass':session.role==='admin'?'Company administrator · permission based':`${session.role} account`}</p></div><Avatar name={session.username}/></div>
      </header>
      <div className="border-b border-slate-200/80 bg-white px-3 py-2 lg:hidden"><div className="flex gap-1 overflow-x-auto">{items.map(item=>{const NavIcon=icons[item.id];return <button key={item.id} onClick={()=>set(item.id)} className={`flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-xs font-semibold ${page===item.id?'bg-indigo-50 text-indigo-700':'text-slate-500 hover:bg-slate-50'}`}><NavIcon size={14}/>{item.label}</button>})}</div></div>
      <main className="mx-auto max-w-375 px-4 py-6 sm:px-7 sm:py-8">{items.length ? <PageContent page={items.some(item=>item.id===page)?page:(items[0]?.id ?? 'overview')} role={session.role}/> : <div className="rounded-2xl border border-slate-200 bg-white p-8 text-sm text-slate-600">No operations are assigned to this account. Ask a company administrator to update its role.</div>}</main>
      <footer className="px-4 pb-6 text-center text-[10px] font-medium text-slate-400 sm:px-7">Ledgerly · Benefit scheme management <span className="mx-1.5">·</span> Secure, clear, accountable</footer>
    </div>
  </div>
}

function PageContent({ page, role }: { page: Page; role: Session['role'] }) {
  if (page === 'accounts') return <AccessControlPage view="accounts"/>
  if (page === 'roles') return <AccessControlPage view="roles"/>
  if (page === 'overview') return <Dashboard role={role}/>
  if (page === 'promoters') return <ResourcePage kind="promoters"/>
  if (page === 'members' && (role === 'admin' || role === 'staff')) return <ResourcePage kind="members"/>
  if (page === 'members' && role === 'promoter') return <MyMembers/>
  if (page === 'schemes') return <ResourcePage kind="schemes"/>
  if (page === 'enrollments') return <EnrollmentPage role={role}/>
  if (page === 'payments') return <PaymentPage role={role}/>
  if (page === 'customers') return <PartyPage partyType="customer"/>
  if (page === 'suppliers') return <PartyPage partyType="supplier"/>
  if (page === 'customerReceipts') return <FinancialLedgerPage direction="receipt"/>
  if (page === 'supplierPayments') return <FinancialLedgerPage direction="payment"/>
  if (page === 'draws') return <DrawPage/>
  if (page === 'incentives') return <IncentivePage role={role}/>
  if (page === 'reports') return <ReportsPage/>
  if (page === 'audit') return <AuditPage/>
  if (page === 'profile') return <MemberProfilePage/>
  return <Dashboard role={role}/>
}

function MyMembers() {
  const { data, error, loading, refresh } = useLoadRows('/members/mine')
  return <div className="page-enter"><PageHeading eyebrow="Your network" title="My members" description="Members assigned to your promoter account." action={<Button variant="outline" onClick={()=>void refresh()}><Activity size={15}/>Refresh</Button>}/><div className="mb-5 grid gap-4 sm:grid-cols-3"><Mini label="Assigned members" value={String(data?.length ?? '—')}/><Mini label="KYC complete" value={String(data?.filter(row=>Boolean(row.kycVerifiedAt)).length ?? '—')}/><Mini label="Active members" value={String(data?.filter(row=>row.active).length ?? '—')}/></div><div className="overflow-hidden rounded-2xl border border-slate-200 bg-white"><div className="border-b border-slate-100 px-5 py-4"><p className="text-sm font-bold">Member roster</p><p className="mt-1 text-xs text-slate-400">Only assigned records are visible to this account.</p></div>{error&&<div className="px-5 py-3 text-sm text-rose-600">{error}</div>}{loading?<div className="py-12 text-center text-sm text-slate-400">Loading member roster…</div>:<DataTable rows={data||[]} columns={['memberNumber','name','phone','kycType','kycVerifiedAt','active']} empty="No members are assigned yet"/>}</div></div>
}
function useLoadRows(path:string){
  const [data,setData]=useState<Row[]|null>(null);const [error,setError]=useState('');const [loading,setLoading]=useState(true)
  const refresh=async()=>{setLoading(true);setError('');try{setData(await api<Row[]>(path))}catch(err){setError(err instanceof Error?err.message:'Could not load records')}finally{setLoading(false)}}
  useEffect(()=>{void refresh()},[path])
  return {data,error,loading,refresh}
}
function Mini({label,value}:{label:string;value:string}){return <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4"><p className="text-xs font-semibold text-slate-500">{label}</p><p className="mt-2 font-display text-xl font-extrabold text-slate-900">{value}</p></div>}

function Brand() { return <div className="flex h-18 items-center gap-3 border-b border-white/10 px-6"><div className="flex size-9 items-center justify-center rounded-xl bg-linear-to-br from-indigo-400 to-violet-600 text-white shadow-lg shadow-indigo-950/40"><CircleDollarSign size={20}/></div><div><p className="font-display text-[15px] font-extrabold tracking-tight text-white">Ledgerly</p><p className="text-[9px] font-bold uppercase tracking-[.2em] text-indigo-300">Benefit scheme</p></div></div> }
function Nav({items,active,onSelect}:{items:{id:Page;label:string;section:string}[];active:Page;onSelect:(page:Page)=>void}) {
  const sections=[...new Set(items.map(item=>item.section))]
  return <nav className="flex-1 overflow-y-auto px-3 py-5">{sections.map(section=><div className="mb-5" key={section}><p className="px-3 pb-2 text-[9px] font-extrabold uppercase tracking-[.18em] text-slate-500">{section}</p><div className="space-y-1">{items.filter(item=>item.section===section).map(item=>{const Icon=icons[item.id];const selected=active===item.id;return <button key={item.id} onClick={()=>onSelect(item.id)} className={`group flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-[12px] font-semibold transition ${selected?'bg-indigo-500/15 text-white shadow-[inset_2px_0_0_#818cf8]':'text-slate-400 hover:bg-white/5 hover:text-slate-200'}`}><Icon size={16} className={selected?'text-indigo-300':'text-slate-500 group-hover:text-slate-300'}/>{item.label}{selected&&<span className="ml-auto size-1.5 rounded-full bg-indigo-300"/>}</button>})}</div></div>)}</nav>
}
function Account({session,onLogout}:{session:Session;onLogout:()=>void}) { return <div className="mt-4 flex items-center gap-3 border-t border-white/10 px-2 pt-4"><Avatar name={session.username} dark/><div className="min-w-0 flex-1"><p className="truncate text-xs font-bold text-white">{session.username}</p><p className="text-[10px] capitalize text-slate-500">{session.role}</p></div><button aria-label="Sign out" onClick={onLogout} className="rounded-lg p-2 text-slate-500 hover:bg-white/10 hover:text-white"><LogOut size={15}/></button></div> }
function Avatar({name,dark=false}:{name:string;dark?:boolean}) { const initials=name.split(/[\s@._-]+/).filter(Boolean).slice(0,2).map(part=>part[0]).join('').toUpperCase();return <div className={`flex size-9 shrink-0 items-center justify-center rounded-full text-xs font-extrabold ${dark?'bg-indigo-500/20 text-indigo-200':'bg-linear-to-br from-amber-100 to-orange-200 text-orange-800 ring-2 ring-white'}`}>{initials||'U'}</div> }

function Login({onLogin}:{onLogin:(username:string,password:string)=>Promise<void>}) {
  const [username,setUsername]=useState('');const [password,setPassword]=useState('');const [error,setError]=useState('');const [busy,setBusy]=useState(false)
  async function submit(event:FormEvent<HTMLFormElement>){event.preventDefault();setError('');setBusy(true);try{await onLogin(username,password)}catch(err){setError(err instanceof Error?err.message:'Could not sign in')}finally{setBusy(false)}}
  return <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#f7f8fb] px-4 py-10"><div className="absolute -left-40 -top-48 size-130 rounded-full bg-indigo-200/45 blur-[100px]"/><div className="absolute -bottom-48 -right-32 size-120 rounded-full bg-violet-200/40 blur-[110px]"/><div className="relative grid w-full max-w-5xl overflow-hidden rounded-[30px] border border-white/80 bg-white shadow-[0_32px_100px_rgba(35,42,74,.12)] lg:grid-cols-[1.08fr_.92fr]">
    <section className="relative hidden min-h-155 flex-col justify-between overflow-hidden bg-[#11182b] p-10 text-white lg:flex"><div className="absolute -right-28 -top-10 size-96 rounded-full border border-white/5"/><div className="absolute -right-4 top-28 size-72 rounded-full border border-white/5"/><Brand/><div className="relative z-10 max-w-md"><BadgeUI/><h1 className="mt-6 font-display text-4xl font-extrabold leading-[1.12] tracking-tight">Every contribution.<br/><span className="text-indigo-300">Clearly accounted for.</span></h1><p className="mt-5 max-w-sm text-sm leading-7 text-slate-400">One considered workspace for member records, monthly payments, draw outcomes, and promoter earnings.</p><div className="mt-10 space-y-4">{[['01','A clear member ledger'],['02','Receipts that are easy to trace'],['03','Every important change recorded']].map(([n,label])=><div className="flex items-center gap-3" key={n}><span className="font-mono text-[10px] text-indigo-300">{n}</span><span className="h-px w-6 bg-slate-700"/><span className="text-xs font-medium text-slate-300">{label}</span></div>)}</div></div><p className="relative z-10 text-[10px] font-semibold tracking-wide text-slate-500">LEDGERLY · BENEFIT SCHEME MANAGEMENT</p></section>
    <section className="flex min-h-150 flex-col justify-center p-6 sm:p-10 lg:p-12"><div className="mb-10 lg:hidden"><div className="mb-5 flex size-10 items-center justify-center rounded-xl bg-indigo-600 text-white"><CircleDollarSign size={21}/></div><p className="font-display text-lg font-extrabold">Ledgerly</p><p className="mt-1 text-xs text-slate-500">Benefit scheme management</p></div><p className="text-[10px] font-extrabold uppercase tracking-[.2em] text-indigo-600">Welcome back</p><h2 className="mt-2 font-display text-3xl font-extrabold tracking-tight text-slate-950">Sign in to Ledgerly</h2><p className="mt-2 text-sm leading-6 text-slate-500">Use the username and password provided by your administrator.</p>
      {error&&<div role="alert" className="mt-6 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</div>}
      <form className="mt-8 space-y-5" onSubmit={submit}><label className="block"><span className="mb-2 block text-xs font-bold text-slate-700">Username</span><Input autoComplete="username" value={username} onChange={e=>setUsername(e.target.value)} required placeholder="Your username" /></label><label className="block"><span className="mb-2 block text-xs font-bold text-slate-700">Password</span><Input type="password" autoComplete="current-password" value={password} onChange={e=>setPassword(e.target.value)} required placeholder="Your password" /></label><Button className="mt-2 h-11 w-full rounded-xl" disabled={busy}>{busy?'Signing in…':'Continue securely'}<ArrowDownToLine size={15} className="-rotate-45"/></Button></form>
      <div className="mt-8 flex items-center justify-center gap-2 text-[11px] text-slate-400"><ShieldCheck size={14} className="text-emerald-600"/>Your session is protected with signed access tokens</div><p className="mt-auto pt-10 text-center text-[10px] text-slate-400">Need access? Ask your scheme administrator to provision an account.</p>
    </section>
  </div></div>
}
function BadgeUI(){return <span className="inline-flex items-center gap-2 rounded-full border border-indigo-400/20 bg-indigo-400/10 px-3 py-1.5 text-[10px] font-bold uppercase tracking-[.14em] text-indigo-200"><span className="size-1.5 rounded-full bg-emerald-400"/>A calmer way to manage schemes</span>}
