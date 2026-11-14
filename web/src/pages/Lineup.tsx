import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type LineupData, type LineupPlayer } from '../api'
import { useOpenPlayer } from '../Inline'
import CourtLineup from '../CourtLineup'
import { Card, Delta, InjuryTag, Page, Q, TierChip, ovrColor } from '../ui'

type Edit = { starters: number[]; minutes: Record<number, number> }

const fromServer = (d: LineupData): Edit => ({
  starters: [...d.starters],
  minutes: Object.fromEntries(d.players.map((p) => [p.id, p.minutes])),
})
const same = (a: Edit, b: Edit) =>
  a.starters.every((v, i) => v === b.starters[i]) && Object.keys(a.minutes).every((k) => a.minutes[+k] === b.minutes[+k])

function problem(d: LineupData, e: Edit): string {
  const vals = d.players.map((p) => e.minutes[p.id] ?? 0)
  const sum = vals.reduce((a, b) => a + b, 0)
  if (vals.some((v) => !Number.isInteger(v) || v < 0 || v > d.maxMinutes)) return `Each player needs 0 to ${d.maxMinutes} minutes.`
  if (e.starters.some((s) => s < 0) || new Set(e.starters).size !== e.starters.length) return 'Pick five different starters.'
  if (sum !== d.total) return sum < d.total ? `Hand out ${d.total - sum} more minutes (total must be ${d.total}).` : `Remove ${sum - d.total} minutes (total must be ${d.total}).`
  if (e.starters.some((s) => (e.minutes[s] ?? 0) < 1)) return 'Every starter needs at least 1 minute.'
  if (vals.filter((v) => v > 0).length < d.minPlayers) return `At least ${d.minPlayers} players must get minutes.`
  return ''
}

export default function Lineup() {
  const q = useQuery({ queryKey: ['lineup'], queryFn: api.lineup })
  return <Q q={q} minH={400}>{(d) => <LineupView d={d} />}</Q>
}

function LineupView({ d }: { d: LineupData }) {
  const qc = useQueryClient()
  const open = useOpenPlayer()
  const base = fromServer(d)
  // null = no edits; the page then always mirrors the latest server data.
  const [edit, setEdit] = useState<Edit | null>(null)
  const cur = edit && !same(edit, base) ? edit : base
  const dirty = cur !== base
  const err = problem(d, cur)
  const sum = d.players.reduce((a, p) => a + (cur.minutes[p.id] ?? 0), 0)
  const byId = new Map(d.players.map((p) => [p.id, p]))

  const done = (r: LineupData) => {
    qc.setQueryData(['lineup'], r)
    qc.invalidateQueries({ queryKey: ['team'] })
    setEdit(null)
  }
  const save = useMutation({
    mutationFn: () => api.setLineup({ starters: cur.starters, minutes: Object.fromEntries(Object.entries(cur.minutes).map(([k, v]) => [k, v])) }),
    onSuccess: done,
  })
  const auto = useMutation({ mutationFn: api.autoLineup, onSuccess: done })
  const busy = save.isPending || auto.isPending
  const serverErr = save.error ?? auto.error

  const change = (f: (e: Edit) => Edit) => { save.reset(); auto.reset(); setEdit(f(cur)) }
  const setMin = (id: number, v: number) => {
    const n = Math.max(0, Math.min(d.maxMinutes, Math.round(Number.isFinite(v) ? v : 0)))
    change((e) => ({ ...e, minutes: { ...e.minutes, [id]: n } }))
  }
  const setStarter = (slot: number, id: number) =>
    change((e) => {
      const s = [...e.starters]
      const j = id >= 0 ? s.indexOf(id) : -1
      if (j >= 0) s[j] = s[slot]
      s[slot] = id
      return { ...e, starters: s }
    })
  const fill = () =>
    change((e) => {
      const m = { ...e.minutes }
      let diff = d.total - sum
      const pool = d.players.filter((p) => !e.starters.includes(p.id) && m[p.id] > 0)
      if (!pool.length) return e
      while (diff !== 0) {
        const ok = pool.filter((p) => (diff > 0 ? m[p.id] < d.maxMinutes : m[p.id] > 1))
        if (!ok.length) break
        const pick = ok.reduce((a, b) => ((diff > 0 ? m[b.id] < m[a.id] : m[b.id] > m[a.id]) ? b : a))
        m[pick.id] += diff > 0 ? 1 : -1
        diff += diff > 0 ? -1 : 1
      }
      return { ...e, minutes: m }
    })

  const startIds = cur.starters.filter((s) => byId.has(s))
  const rows: LineupPlayer[] = [...startIds.map((s) => byId.get(s)!), ...d.players.filter((p) => !startIds.includes(p.id))]
  const chip = dirty ? ['Unsaved changes', 'bg-amber-400/20 text-amber-400 ring-amber-400/50']
    : d.custom ? ['Custom', 'bg-accent/25 text-accent ring-accent/60'] : ['Automatic', 'bg-panel2 text-muted ring-line']
  const off = sum !== d.total
  const left = d.total - sum

  return (
    <Page
      title={<span className="flex flex-wrap items-center gap-3">Lineup<span className={`rounded px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ring-1 ${chip[1]}`}>{chip[0]}</span></span>}
      right={
        <div className="flex flex-wrap items-center gap-2">
          <button className="btn" disabled={busy || !dirty} onClick={() => { save.reset(); setEdit(null) }}>Undo changes</button>
          <button className="btn" disabled={busy} onClick={() => { setEdit(null); auto.mutate() }}>Reset to automatic</button>
          <button className="btn btn-accent" disabled={busy || !dirty || !!err} onClick={() => save.mutate()}>Save lineup</button>
        </div>
      }
    >
      {(err && dirty) && <p className="mb-3 text-sm text-amber-400" role="status">{err}</p>}
      {serverErr && <p className="mb-3 text-sm text-loss" role="alert">{serverErr instanceof Error ? serverErr.message : String(serverErr)}</p>}

      <Card title="Minutes" className="mb-4">
        <div className="px-2 pb-2">
          <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
            <span>Minutes used <b className={`tabular-nums ${off ? 'text-amber-400' : ''}`}>{sum}</b> of {d.total}
              <span className={`ml-2 ${off ? 'text-amber-400' : 'text-muted'}`}>{left > 0 ? `${left} left to hand out` : left < 0 ? `${-left} to remove` : 'all handed out'}</span></span>
            <button className="btn" disabled={busy} onClick={fill}>Fill the rest evenly</button>
          </div>
          <div className="mt-2 h-2.5 overflow-hidden rounded-full bg-panel2" role="progressbar" aria-valuemin={0} aria-valuemax={d.total} aria-valuenow={sum} aria-label="Minutes used">
            <div className={`h-full rounded-full ${off ? 'bg-amber-400' : 'bg-win'}`} style={{ width: `${Math.min(100, (100 * sum) / d.total)}%` }} />
          </div>
        </div>
      </Card>

      <Card title="Court" className="mb-4">
        <CourtLineup slots={d.slots} starters={cur.starters} players={d.players} minutes={cur.minutes} onSet={setStarter} />
      </Card>

      <Card title="Starters" className="mb-4">
        <div className="flex flex-wrap gap-3 px-2 pb-2">
          {d.slots.map((slot, i) => {
            const id = cur.starters[i]
            const zero = id >= 0 && (cur.minutes[id] ?? 0) < 1
            return (
              <div key={slot} className="min-w-[10rem] flex-1">
                <label htmlFor={`slot-${i}`} className="mb-1 block text-[11px] font-bold uppercase tracking-wider text-muted">{slot}</label>
                <select id={`slot-${i}`} className="w-full rounded-md border border-line bg-panel2 px-2 py-1.5" value={id} onChange={(e) => setStarter(i, Number(e.target.value))}>
                  <option value={-1}>(empty)</option>
                  {d.players.map((p) => <option key={p.id} value={p.id}>{p.name} · {p.pos} · {p.ovr}</option>)}
                </select>
                {zero && <div className="mt-1 text-xs text-amber-400">Starter has 0 minutes</div>}
              </div>
            )
          })}
        </div>
      </Card>

      <Card title="Rotation">
        <div className="overflow-x-auto">
          <table className="tbl">
            <thead>
              <tr><th>Player</th><th>Pos</th><th className="r">Age</th><th className="r">OVR/POT</th><th className="r">GP · MPG</th><th className="r">Auto</th><th>Target minutes</th></tr>
            </thead>
            <tbody>
              {rows.map((p) => {
                const v = cur.minutes[p.id] ?? 0
                const isStarter = cur.starters.includes(p.id)
                const young = p.age <= d.dev.maxAge
                const tone = v >= d.dev.fullMinutes ? 'bg-win/20 text-win ring-win/50' : v <= d.dev.noneMinutes ? 'bg-amber-400/20 text-amber-400 ring-amber-400/50' : 'bg-panel2 text-muted ring-line'
                return (
                  <tr key={p.id}>
                    <td>
                      <button className="link font-medium" onClick={() => open(p.id)}>{p.name}</button>
                      <TierChip tier={p.tier} /><InjuryTag injury={p.injury} />
                      {isStarter && <span className="ml-1.5 text-[10px] font-semibold uppercase text-accent">Starter</span>}
                      {young && <span className={`ml-1.5 rounded px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide ring-1 ${tone}`}>Developing</span>}
                      {isStarter && v < 1 && <span className="ml-1.5 text-xs text-amber-400">needs minutes</span>}
                    </td>
                    <td>{p.pos}</td>
                    <td className="r">{p.age}</td>
                    <td className="r"><b style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b><Delta value={p.ovrChange} scope={p.changeScope} /> / {p.pot}<Delta value={p.potChange} scope={p.changeScope} /></td>
                    <td className="r">{p.gp} · {p.mpg.toFixed(1)}</td>
                    <td className="r text-muted">{p.autoMinutes}</td>
                    <td>
                      <div className="flex items-center gap-1.5">
                        <button className="btn !px-2 !py-0.5" aria-label={`One minute less for ${p.name}`} disabled={v <= 0} onClick={() => setMin(p.id, v - 1)}>-1</button>
                        <input type="number" min={0} max={d.maxMinutes} step={1} aria-label={`Target minutes for ${p.name}`} value={v}
                          onChange={(e) => setMin(p.id, e.target.valueAsNumber)}
                          className="w-14 rounded-md border border-line bg-panel2 px-1.5 py-0.5 text-right tabular-nums" />
                        <button className="btn !px-2 !py-0.5" aria-label={`One minute more for ${p.name}`} disabled={v >= d.maxMinutes} onClick={() => setMin(p.id, v + 1)}>+1</button>
                        <input type="range" min={0} max={d.maxMinutes} step={1} aria-label={`Minutes slider for ${p.name}`} value={v}
                          onChange={(e) => setMin(p.id, Number(e.target.value))} className="w-28 accent-[var(--color-accent)]" />
                        {!dirty && p.nextMinutes !== p.minutes && <span className="text-xs text-muted">next game: {p.nextMinutes}</span>}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        <p className="px-2 pt-2 pb-1 text-xs text-muted">
          Players aged {d.dev.maxAge} or under develop a little faster when they play about {d.dev.fullMinutes}+ minutes a game and slower when they play under {d.dev.noneMinutes}.
        </p>
      </Card>
    </Page>
  )
}
