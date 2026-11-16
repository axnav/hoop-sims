import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Filter } from 'lucide-react'
import { api, type SearchRow } from '../api'
import { useOpenPlayer } from '../Inline'
import { Avatar, BadgeCount, Delta, ErrorBox, InjuryTag, MoraleChip, Page, Spinner, TradeRequestTag, groupLabel, groupOrder, money, ovrColor } from '../ui'

const PER = 50
const positions = ['PG', 'SG', 'SF', 'PF', 'C']
const moods = ['Furious', 'Unhappy', 'Content', 'Happy', 'Thrilled']
const filterKeys = ['q', 'team', 'pos', 'minAge', 'maxAge', 'minOvr', 'maxOvr', 'maxSalary', 'yearsLeft', 'badge', 'tradeRequest', 'morale']

type SortKey = 'ovr' | 'pot' | 'age' | 'salary' | 'name' | 'badges'
const defaultDir = (k: string) => (k === 'name' || k === 'age' ? 'asc' : 'desc')

// A text or number box that waits a moment before writing to the URL.
function Field({ label, value, onChange, type = 'text', width = 'w-20', placeholder }: { label: string; value: string; onChange: (v: string) => void; type?: string; width?: string; placeholder?: string }) {
  const [v, setV] = useState(value)
  useEffect(() => setV(value), [value])
  useEffect(() => {
    if (v === value) return
    const t = setTimeout(() => onChange(v), 300)
    return () => clearTimeout(t)
  }, [v]) // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <label className="flex flex-col gap-0.5 text-[11px] font-semibold uppercase tracking-wider text-muted">{label}
      <input aria-label={label} type={type} inputMode={type === 'number' ? 'numeric' : undefined} value={v} placeholder={placeholder} onChange={(e) => setV(e.target.value)}
        className={`${width} rounded border border-line bg-panel px-2 py-1 text-[13px] font-normal normal-case tracking-normal text-ink`} />
    </label>
  )
}

export default function Players() {
  const [sp, setSp] = useSearchParams()
  const open = useOpenPlayer()
  const [showFilters, setShowFilters] = useState(false)
  const get = (k: string) => sp.get(k) ?? ''
  const set = (changes: Record<string, string>, keepOffset = false) =>
    setSp((p) => {
      for (const [k, v] of Object.entries(changes)) v ? p.set(k, v) : p.delete(k)
      if (!keepOffset) p.delete('offset')
      return p
    }, { replace: true })

  const sort = (get('sort') || 'ovr') as SortKey
  const dir = get('dir') || defaultDir(sort)
  const offset = Math.max(0, Number(get('offset')) || 0)
  const qs = new URLSearchParams()
  for (const k of filterKeys) if (get(k)) qs.set(k, get(k))
  qs.set('sort', sort); qs.set('dir', dir); qs.set('limit', String(PER)); qs.set('offset', String(offset))
  const res = useQuery({ queryKey: ['players', qs.toString()], queryFn: () => api.players(qs.toString()), placeholderData: (d) => d })
  const teams = useQuery({ queryKey: ['teams-list'], queryFn: api.teams, staleTime: 60000 })
  const defs = useQuery({ queryKey: ['badges'], queryFn: api.badges, staleTime: Infinity })
  const active = filterKeys.filter((k) => get(k)).length

  const sel = 'rounded border border-line bg-panel px-2 py-1 text-[13px] font-normal normal-case tracking-normal text-ink'
  const lab = 'flex flex-col gap-0.5 text-[11px] font-semibold uppercase tracking-wider text-muted'
  const filters = (
    <div className="flex flex-wrap items-end gap-x-3 gap-y-2">
      <Field label="Name" value={get('q')} onChange={(v) => set({ q: v })} width="w-40" placeholder="Any name" />
      <label className={lab}>Team
        <select aria-label="Team" value={get('team')} onChange={(e) => set({ team: e.target.value })} className={sel}>
          <option value="">All teams</option>
          <option value="fa">Free agents</option>
          {(teams.data?.teams ?? []).map((t) => <option key={t.team.id} value={t.team.id}>{t.team.fullName}</option>)}
        </select>
      </label>
      <label className={lab}>Position
        <select aria-label="Position" value={get('pos')} onChange={(e) => set({ pos: e.target.value })} className={sel}>
          <option value="">Any</option>
          {positions.map((x) => <option key={x}>{x}</option>)}
        </select>
      </label>
      <Field label="Age min" type="number" value={get('minAge')} onChange={(v) => set({ minAge: v })} width="w-16" />
      <Field label="Age max" type="number" value={get('maxAge')} onChange={(v) => set({ maxAge: v })} width="w-16" />
      <Field label="Ovr min" type="number" value={get('minOvr')} onChange={(v) => set({ minOvr: v })} width="w-16" />
      <Field label="Ovr max" type="number" value={get('maxOvr')} onChange={(v) => set({ maxOvr: v })} width="w-16" />
      <Field label="Max salary ($K)" type="number" value={get('maxSalary')} onChange={(v) => set({ maxSalary: v })} width="w-24" placeholder="e.g. 10000" />
      <Field label="Years left" type="number" value={get('yearsLeft')} onChange={(v) => set({ yearsLeft: v })} width="w-16" />
      <label className={lab}>Badge
        <select aria-label="Badge" value={get('badge')} onChange={(e) => set({ badge: e.target.value })} className={`${sel} max-w-[12rem]`}>
          <option value="">Any</option>
          {groupOrder.map((g) => {
            const list = (defs.data?.badges ?? []).filter((b) => b.group === g)
            return list.length === 0 ? null : <optgroup key={g} label={groupLabel[g]}>{list.map((b) => <option key={b.key} value={b.key}>{b.name}</option>)}</optgroup>
          })}
        </select>
      </label>
      <label className={lab}>Mood
        <select aria-label="Mood" value={get('morale')} onChange={(e) => set({ morale: e.target.value })} className={sel}>
          <option value="">Any</option>
          {moods.map((x) => <option key={x}>{x}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-1.5 pb-1 text-[13px] text-ink">
        <input type="checkbox" checked={get('tradeRequest') === '1'} onChange={(e) => set({ tradeRequest: e.target.checked ? '1' : '' })} /> Wants out
      </label>
      {active > 0 && <button className="btn pb-1" onClick={() => setSp({}, { replace: true })}>Clear</button>}
    </div>
  )

  const head = (key: SortKey | null, label: string, cls = '', right = false) => {
    const on = key === sort
    return (
      <th className={`${key ? 'cursor-pointer select-none' : ''} ${right ? 'r' : ''} ${cls}`} aria-sort={on ? (dir === 'asc' ? 'ascending' : 'descending') : undefined}
        onClick={key ? () => set({ sort: key === 'ovr' ? '' : key, dir: on ? (dir === 'asc' ? 'desc' : 'asc') : '' }) : undefined}>
        <span className="inline-flex items-center gap-1">{label}{on ? (dir === 'asc' ? <ArrowUp size={11} /> : <ArrowDown size={11} />) : <span className="w-[11px]" />}</span>
      </th>
    )
  }
  const small = 'max-[699px]:hidden'
  const row = (r: SearchRow) => (
    <tr key={r.id} className="cursor-pointer" onClick={() => open(r.id)}>
      <td>
        <Avatar id={r.id} name={r.name} photo={r.photo} color={r.team?.primary} size={28} /> <span className="link font-medium">{r.name}</span>
        <InjuryTag injury={r.injury} /><span className="max-[699px]:hidden"><BadgeCount n={r.badgeCount} /></span>{r.tradeRequest && <TradeRequestTag />}
      </td>
      <td onClick={(e) => e.stopPropagation()}>{r.team ? <Link to={`/team/${r.team.id}`} className="link">{r.team.abbr}</Link> : <span className="text-muted">FA</span>}</td>
      <td>{r.pos}</td>
      <td className="r">{r.age}</td>
      <td className="r font-bold" style={{ color: ovrColor(r.ovr) }}>{r.ovr}<Delta value={r.ovrChange} scope={r.changeScope} /></td>
      <td className={`r ${small}`} style={{ color: ovrColor(r.pot) }}>{r.pot}</td>
      <td className={`r ${small}`}>{r.team ? money(r.salary) : '-'}</td>
      <td className={`r ${small}`}>{r.team ? r.years : '-'}</td>
      <td className={small}><MoraleChip morale={r.morale} /></td>
    </tr>
  )

  const total = res.data?.total ?? 0
  const from = total === 0 ? 0 : offset + 1
  const to = Math.min(total, offset + PER)
  return (
    <Page title="Players" right={<span className="text-sm text-muted tabular-nums">{res.data ? `${total} ${total === 1 ? 'player' : 'players'}` : ''}</span>}>
      <div className="card mb-4 p-3">
        <div className="min-[700px]:hidden">
          <button className="btn" onClick={() => setShowFilters((s) => !s)} aria-expanded={showFilters}><Filter size={15} /> Filters{active > 0 && ` (${active})`}</button>
        </div>
        <div className={`${showFilters ? 'mt-3 block' : 'max-[699px]:hidden'}`}>{filters}</div>
      </div>
      {res.isError && !res.data && <ErrorBox error={res.error} />}
      {!res.data && !res.isError && <div style={{ minHeight: 400 }}><Spinner /></div>}
      {res.data && (
        <div className="card overflow-x-auto p-2">
          <table className="tbl">
            <thead>
              <tr>
                {head('name', 'Player')}{head(null, 'Team')}{head(null, 'Pos')}{head('age', 'Age', '', true)}{head('ovr', 'Ovr', '', true)}
                {head('pot', 'Pot', small, true)}{head('salary', 'Salary', small, true)}{head(null, 'Yrs', `r ${small}`)}{head(null, 'Mood', small)}
              </tr>
            </thead>
            <tbody>
              {res.data.players.map(row)}
              {res.data.players.length === 0 && <tr><td colSpan={9} className="text-muted">No players match these filters.</td></tr>}
            </tbody>
          </table>
        </div>
      )}
      {res.data && total > PER && (
        <div className="mt-3 flex items-center justify-between gap-3 text-sm text-muted">
          <button className="btn" disabled={offset === 0} onClick={() => set({ offset: String(Math.max(0, offset - PER)) }, true)}>Previous</button>
          <span className="tabular-nums">{from}-{to} of {total}</span>
          <button className="btn" disabled={to >= total} onClick={() => set({ offset: String(offset + PER) }, true)}>Next</button>
        </div>
      )}
    </Page>
  )
}
