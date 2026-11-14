import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, LayoutGroup, motion } from 'motion/react'
import { Check, Lock, Plus, Sparkles, X } from 'lucide-react'
import { api, type DealMove, type Team, type TradeAssetRow, type TradePickRow, type TradeResult, type TradeVerdict } from '../api'
import { useFlow } from '../flow'
import { TxItem } from '../TxItem'
import { Card, ErrorBox, Page, PlayerLink, Q, Spinner, TeamBadge, ovrColor } from '../ui'

// One selected asset in the deal in progress. `key` identifies the asset.
type Sel = { key: string; label: string; from: number; to: number; playerId?: number; pick?: { year: number; round: number; original: number } }
const pkey = (id: number) => `p${id}`
const kkey = (p: { year: number; round: number; original: number }) => `k${p.year}-${p.round}-${p.original}`
const toMove = (m: Sel): DealMove => (m.pick ? { from: m.from, to: m.to, pick: m.pick } : { from: m.from, to: m.to, playerId: m.playerId! })

function Row({ p, maxV, disabled, onPick, hl }: { p: TradeAssetRow; maxV: number; disabled: boolean; onPick: () => void; hl?: boolean }) {
  return (
    <motion.li
      layout layoutId={`p-${p.id}`} transition={{ type: 'spring', stiffness: 420, damping: 36 }}
      className={`flex items-center gap-2 border-b border-line/50 px-2 py-1.5 ${disabled ? 'opacity-60' : 'cursor-pointer hover:bg-panel2'} ${hl ? 'bg-accent/15 ring-1 ring-accent' : ''}`}
      onClick={() => !disabled && onPick()}
      title={p.untouchable ? 'A core player: the team will want a lot for him.' : undefined}
    >
      <span className="w-7 text-xs text-muted">{p.pos}</span>
      <span className="min-w-0 flex-1 truncate font-medium">
        <PlayerLink id={p.id} name={p.name} />
        {p.untouchable && <Lock size={12} className="ml-1.5 inline text-accent" aria-label="Core player" />}
      </span>
      <span className="w-8 text-right text-xs text-muted tabular-nums">{p.age}y</span>
      <b className="w-7 text-right tabular-nums" style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b>
      <span className="w-7 text-right text-muted tabular-nums">{p.pot}</span>
      <span className="h-1.5 w-14 overflow-hidden rounded bg-panel2" title={`Trade value ${p.value}`}>
        <span className="block h-full rounded bg-accent" style={{ width: `${Math.max(4, (100 * p.value) / maxV)}%` }} />
      </span>
    </motion.li>
  )
}

function PickRow({ k, maxV, disabled, onPick }: { k: TradePickRow; maxV: number; disabled: boolean; onPick: () => void }) {
  return (
    <li
      className={`flex items-center gap-2 border-b border-line/50 px-2 py-1.5 ${disabled ? 'opacity-60' : 'cursor-pointer hover:bg-panel2'}`}
      onClick={() => !disabled && onPick()}
    >
      <span className="w-7 text-xs text-muted">Pick</span>
      <span className="min-w-0 flex-1 truncate font-medium">{k.label}</span>
      <span className="h-1.5 w-14 overflow-hidden rounded bg-panel2" title={`Trade value ${k.value}`}>
        <span className="block h-full rounded bg-accent" style={{ width: `${Math.max(4, (100 * k.value) / maxV)}%` }} />
      </span>
    </li>
  )
}

function DealMeter({ gets, gives, needed, accept }: { gets: number; gives: number; needed: number; accept: boolean }) {
  const need = gives * needed
  const fill = need > 0 ? Math.min(1, gets / need) : 0
  return (
    <div>
      <div className="h-3 overflow-hidden rounded-full bg-panel2">
        <motion.div className="h-full rounded-full" animate={{ width: `${fill * 100}%`, background: accept ? '#3ddc84' : '#ff5a67' }} transition={{ duration: 0.5 }} />
      </div>
      <div className="mt-1 flex justify-between gap-2 text-[11px] text-muted">
        <span>They receive {gets.toFixed(1)}</span>
        <span>They want at least {need.toFixed(1)} (they give {gives.toFixed(1)})</span>
      </div>
    </div>
  )
}

function DealMeterFromVerdict({ v }: { v: TradeVerdict }) {
  return v.valid ? <DealMeter gets={v.theyGet} gives={v.theyGive} needed={v.needed} accept={v.accept} /> : null
}

function Summary({ r, onClose }: { r: TradeResult; onClose: () => void }) {
  const mine = r.state.user.team.id
  const sent = r.moved.filter((m) => m.from.id === mine)
  const got = r.moved.filter((m) => m.to.id === mine)
  const sentPicks = (r.picks ?? []).filter((m) => m.from.id === mine)
  const gotPicks = (r.picks ?? []).filter((m) => m.to.id === mine)
  const others = (r.teams && r.teams.length > 2 ? r.teams : [r.teamA, r.teamB]).filter((t) => t.id !== mine).map((t) => t.abbr).join(', ')
  const card = (k: string, text: string, dir: number) => (
    <motion.div key={k} className="card px-3 py-2 font-semibold" initial={{ x: dir * 160, opacity: 0.2 }} animate={{ x: 0, opacity: 1 }} transition={{ type: 'spring', stiffness: 160, damping: 18 }}>
      {text}
    </motion.div>
  )
  return (
    <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card mb-4 border-win/50 p-4">
      <div className="mb-3 flex items-center gap-2 text-lg font-bold text-win"><Check size={18} />Trade complete</div>
      <div className="grid gap-4 md:grid-cols-2">
        <div><div className="h2 mb-1">Sent to {others}</div><div className="space-y-1">{sent.map((m) => card(`p${m.playerId}`, m.name, 1))}{sentPicks.map((p) => card(`k${p.label}-${p.original.id}`, p.label, 1))}</div></div>
        <div><div className="h2 mb-1">Received</div><div className="space-y-1">{got.map((m) => card(`p${m.playerId}`, m.name, -1))}{gotPicks.map((p) => card(`k${p.label}-${p.original.id}`, p.label, -1))}</div></div>
      </div>
      {(r.released.length > 0 || r.signed.length > 0) && (
        <ul className="mt-3 space-y-0.5 text-[13px] text-muted">
          {r.released.map((x) => <li key={x.playerId}>{x.name} was released by {x.from.fullName} to keep the roster at 13.</li>)}
          {r.signed.map((x) => <li key={x.playerId}>{x.to.fullName} signed free agent {x.name} to keep the roster at 13.</li>)}
        </ul>
      )}
      <button className="btn mt-3" onClick={onClose}>Done</button>
    </motion.div>
  )
}

export default function Trade() {
  const [sp] = useSearchParams()
  return <TradeScreen key={`${sp.get('team')}-${sp.get('get')}`} />
}

function TradeScreen() {
  const { state } = useFlow()
  const qc = useQueryClient()
  const [sp] = useSearchParams()
  const me = state!.user.team.id
  const status = useQuery({ queryKey: ['trade-status', state?.phase, state?.day], queryFn: api.tradeStatus })
  const meta = useQuery({ queryKey: ['meta'], queryFn: api.meta })
  const teams: Team[] = useMemo(() => (meta.data ? (state!.mode === 'real' ? meta.data.realTeams : meta.data.fictionalTeams) : []), [meta.data, state])
  // the computer teams in the deal: one, or two when a third team is added
  const [partners, setPartners] = useState<number[]>(() => {
    const t = Number(sp.get('team'))
    return [Number.isInteger(t) && sp.get('team') && t !== me ? t : me === 0 ? 1 : 0]
  })
  const [rawMoves, setRawMoves] = useState<Sel[]>([])
  const [preview, setPreview] = useState<{ moves: Sel[]; verdict: TradeVerdict; added: Set<string> } | null>(null)
  const [done, setDone] = useState<TradeResult | null>(null)
  const [notice, setNotice] = useState('')
  const [pending, setPending] = useState<number | null>(() => {
    const g = Number(sp.get('get'))
    return sp.get('get') && Number.isInteger(g) ? g : null
  })

  const ids = [me, ...partners]
  const blockKey = (id: number) => ['trade-block', id, state?.phase, state?.day, done?.year, done?.day, done?.moved.length]
  const q0 = useQuery({ queryKey: blockKey(ids[0]), queryFn: () => api.tradeBlock(ids[0]) })
  const q1 = useQuery({ queryKey: blockKey(ids[1]), queryFn: () => api.tradeBlock(ids[1]) })
  const q2 = useQuery({ queryKey: blockKey(ids[2] ?? -1), queryFn: () => api.tradeBlock(ids[2]), enabled: ids.length > 2 })
  const qs = [q0, q1, q2].slice(0, ids.length)
  const txQ = useQuery({ queryKey: ['transactions'], queryFn: api.transactions })
  // arriving from the dashboard's offseason-trades list (#transactions)
  useEffect(() => {
    if (window.location.hash === '#transactions' && txQ.data) document.getElementById('transactions')?.scrollIntoView({ behavior: 'smooth' })
  }, [txQ.data])
  const open = status.data?.open ?? false
  const max = status.data?.maxPlayers ?? 3
  const maxPicks = status.data?.maxPicks ?? 2
  const maxTeams = status.data?.maxTeams ?? 3

  // With two teams every asset goes to the other team; with three, `to` is what the user chose.
  const normalize = (m: Sel): Sel => (ids.length === 2 ? { ...m, to: m.from === me ? partners[0] : me } : m)
  const moves = useMemo(() => rawMoves.map(normalize), [rawMoves, partners.join(',')]) // eslint-disable-line react-hooks/exhaustive-deps
  const shown = preview ? preview.moves : moves
  const wire = shown.map(toMove)
  const wireKey = JSON.stringify(wire)

  // the pre-selected player when arriving with ?get=
  useEffect(() => {
    if (pending === null) return
    const row = q1.data?.players.find((p) => p.id === pending)
    if (!row) return
    setPending(null)
    setRawMoves((m) => (m.length === 0 ? [{ key: pkey(row.id), label: row.name, from: partners[0], to: me, playerId: row.id }] : m))
  }, [pending, q1.data]) // eslint-disable-line react-hooks/exhaustive-deps

  const [deb, setDeb] = useState(wireKey)
  useEffect(() => {
    const t = setTimeout(() => setDeb(wireKey), 250)
    return () => clearTimeout(t)
  }, [wireKey])
  const ready = open && wire.length > 0
  const evalQ = useQuery({
    queryKey: ['deal-eval', deb, state?.day],
    queryFn: () => api.dealEvaluate(JSON.parse(deb) as DealMove[]),
    enabled: ready && !preview,
    staleTime: 0,
  })
  const settled = deb === wireKey
  const deal = !preview && ready ? evalQ.data : undefined

  const hasPicks = shown.some((m) => m.pick)
  const twoTeamNoPicks = ids.length === 2 && !hasPicks
  const counter = useMutation({
    mutationFn: () => api.tradeCounter({ team: partners[0], give: moves.filter((m) => m.from === me).map((m) => m.playerId!), get: moves.filter((m) => m.from !== me).map((m) => m.playerId!) }),
    onSuccess: (r) => {
      if (r.found) {
        const label = (id: number) => q0.data?.players.find((p) => p.id === id)?.name ?? q1.data?.players.find((p) => p.id === id)?.name ?? `#${id}`
        const old = new Set(moves.map((m) => m.key))
        const mk = (id: number, from: number, to: number): Sel => ({ key: pkey(id), label: label(id), from, to, playerId: id })
        const next = [...r.give.map((id) => mk(id, me, partners[0])), ...r.get.map((id) => mk(id, partners[0], me))]
        setPreview({ moves: next, verdict: r.verdict, added: new Set(next.filter((m) => !old.has(m.key)).map((m) => m.key)) }); setNotice('')
      } else setNotice('They would not do a deal with those players, whatever else you added.')
    },
  })
  const exec = useMutation({
    mutationFn: () => api.dealExecute(wire),
    onSuccess: (r) => {
      setDone(r); setRawMoves([]); setPreview(null)
      qc.setQueryData(['state'], r.state)
      qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'state' })
    },
  })

  const reset = () => { setPreview(null); setNotice(''); setDone(null); exec.reset() }
  const outCount = (team: number, pick: boolean) => rawMoves.filter((m) => m.from === team && !!m.pick === pick).length
  const defaultTo = (from: number) => (from === me ? partners[0] : me)
  const togglePlayer = (from: number, p: TradeAssetRow) => {
    reset()
    const key = pkey(p.id)
    setRawMoves((ms) => {
      if (ms.some((m) => m.key === key)) return ms.filter((m) => m.key !== key)
      if (outCount(from, false) >= max) return ms
      return [...ms, { key, label: p.name, from, to: defaultTo(from), playerId: p.id }]
    })
  }
  const togglePick = (from: number, k: TradePickRow) => {
    reset()
    const pick = { year: k.year, round: k.round, original: k.original.id }
    const key = kkey(pick)
    setRawMoves((ms) => {
      if (ms.some((m) => m.key === key)) return ms.filter((m) => m.key !== key)
      if (outCount(from, true) >= maxPicks) return ms
      return [...ms, { key, label: k.label, from, to: defaultTo(from), pick }]
    })
  }
  const setDest = (key: string, to: number) => { reset(); setRawMoves((ms) => ms.map((m) => (m.key === key ? { ...m, to } : m))) }
  const removeKey = (key: string) => { reset(); setRawMoves((ms) => ms.filter((m) => m.key !== key)) }
  const pickPartner = (slot: number, id: number) => {
    if (partners[slot] === id) return
    const old = partners[slot]
    setPartners((p) => p.map((x, i) => (i === slot ? id : x)))
    setRawMoves((ms) => ms.filter((m) => m.from !== old && m.to !== old))
    reset()
  }
  const addThird = () => {
    const free = teams.find((t) => t.id !== me && !partners.includes(t.id))
    if (free) { setPartners((p) => [...p, free.id]); reset() }
  }
  const removeThird = () => {
    const gone = partners[1]
    setRawMoves((ms) => ms.filter((m) => m.from !== gone && m.to !== gone).map((m) => ({ ...m, to: m.from === me ? partners[0] : me })))
    setPartners((p) => p.slice(0, 1)); reset()
  }

  const allVals = qs.flatMap((q) => [...(q.data?.players ?? []).map((p) => p.value), ...(q.data?.picks ?? []).map((k) => k.value)])
  const maxV = Math.max(1, ...allVals)
  const teamOf = (id: number) => teams.find((t) => t.id === id)
  const selKeys = new Set(shown.map((m) => m.key))
  const canPropose = open && !!deal?.valid && deal.accept && settled && !exec.isPending && !preview
  const refused = !!deal?.valid && !deal.accept && !preview
  const lock = open && !preview

  const column = (slot: number) => {
    const id = ids[slot], q = qs[slot]
    const team = slot === 0 ? undefined : teamOf(id)
    return (
      <Card key={id} title={slot === 0 ? 'Your roster' : team ? team.fullName : 'Partner'}
        right={slot === 0 ? <span className="text-[11px] text-muted">Pos · age · OVR · POT · value</span> : <span className="flex items-center gap-1 text-[11px] text-muted"><Lock size={11} />core players cost a lot</span>}>
        <Q q={q}>{() => (
          <>
            <ul>{q.data!.players.filter((p) => !selKeys.has(pkey(p.id))).map((p) => <Row key={p.id} p={p} maxV={maxV} disabled={!lock} onPick={() => togglePlayer(id, p)} />)}</ul>
            {q.data!.picks.length > 0 && <div className="h2 px-2 pt-3">Draft picks</div>}
            <ul>{q.data!.picks.filter((k) => !selKeys.has(kkey({ year: k.year, round: k.round, original: k.original.id }))).map((k) => <PickRow key={kkey({ year: k.year, round: k.round, original: k.original.id })} k={k} maxV={maxV} disabled={!lock} onPick={() => togglePick(id, k)} />)}</ul>
          </>
        )}</Q>
      </Card>
    )
  }

  const dealRow = (m: Sel) => {
    const from = teamOf(m.from)
    const opts = ids.filter((t) => t !== m.from)
    return (
      <li key={m.key} className={`flex flex-wrap items-center gap-2 border-b border-line/50 px-2 py-1.5 text-[13px] ${preview?.added.has(m.key) ? 'bg-accent/15 ring-1 ring-accent' : ''}`}>
        {from && <TeamBadge team={from} size={16} />}
        <span className="min-w-0 flex-1 truncate font-medium">{m.label}{m.pick && <span className="ml-1 text-xs text-muted">(pick)</span>}</span>
        {ids.length > 2 && !preview && (
          <label className="flex items-center gap-1 text-xs text-muted">to
            <select className="rounded border border-line bg-panel2 px-1 py-0.5 text-foreground" value={m.to} onChange={(e) => setDest(m.key, Number(e.target.value))}>
              {opts.map((t) => <option key={t} value={t}>{t === me ? 'You' : teamOf(t)?.abbr ?? t}</option>)}
            </select>
          </label>
        )}
        {ids.length === 2 && <span className="text-xs text-muted">to {m.to === me ? 'you' : teamOf(m.to)?.abbr}</span>}
        {open && !preview && <button className="text-muted hover:text-loss" aria-label="Remove" onClick={() => removeKey(m.key)}><X size={14} /></button>}
      </li>
    )
  }

  return (
    <Page title="Trades">
      {status.data && !open && (
        <div className="card mb-4 border-loss/50 p-4">
          <div className="font-bold text-loss">The trade window is closed</div>
          <div className="text-muted">{status.data.reason} {status.data.reopens}</div>
        </div>
      )}
      {done && <Summary r={done} onClose={() => setDone(null)} />}
      {partners.map((pid, slot) => (
        <div key={slot} className="mb-3 flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-xs text-muted">{partners.length > 1 ? `Team ${slot + 1}` : 'Trade with'}</span>
          {teams.filter((t) => t.id !== me && (t.id === pid || !partners.includes(t.id))).map((t) => (
            <button key={t.id} onClick={() => pickPartner(slot, t.id)} className={`rounded-md p-0.5 ${t.id === pid ? 'ring-2 ring-accent' : 'opacity-70 hover:opacity-100'}`} title={t.fullName}>
              <TeamBadge team={t} size={22} />
            </button>
          ))}
          {slot === 1 && <button className="btn ml-2" onClick={removeThird}><X size={14} />Remove third team</button>}
        </div>
      ))}
      {partners.length < Math.min(2, maxTeams - 1) && open && (
        <button className="btn mb-3" onClick={addThird}><Plus size={14} />Add a third team</button>
      )}
      <LayoutGroup>
        <div className={`grid grid-cols-1 gap-4 ${ids.length > 2 ? 'lg:grid-cols-3' : 'lg:grid-cols-2'} ${open ? '' : 'pointer-events-none'}`} style={open ? undefined : { opacity: 0.85 }}>
          {ids.map((_, slot) => column(slot))}
        </div>
        <div className="card my-4 p-3">
          <div className="min-h-[132px] rounded-lg border border-dashed border-line p-2">
            <div className="h2 mb-1 flex justify-between">
              <span>The deal</span>
              <span>{ids.map((t) => `${t === me ? 'You' : teamOf(t)?.abbr ?? t} ${outCount(t, false)}/${max}p ${outCount(t, true)}/${maxPicks}k`).join(' · ')}</span>
            </div>
            <ul>{shown.map(dealRow)}</ul>
            {shown.length === 0 && <div className="py-4 text-center text-xs text-muted">Click players or picks to add them</div>}
          </div>
          <div className="mt-3 min-h-[88px]">
            {preview && <DealMeterFromVerdict v={preview.verdict} />}
            {deal && deal.valid && (
              <div className="space-y-3">
                {deal.teams.map((t) => (
                  <div key={t.team.id}>
                    <div className="mb-1 flex items-center gap-2 text-[13px]"><TeamBadge team={t.team} size={16} /><span className={t.accept ? 'text-win' : 'text-loss'}>{t.reason}</span></div>
                    <DealMeter gets={t.gets} gives={t.gives} needed={t.needed} accept={t.accept} />
                  </div>
                ))}
              </div>
            )}
            <div className="mt-2 text-[13px]">
              {preview && <div className="mb-1 flex items-center gap-1 font-semibold text-accent"><Sparkles size={14} />This is what it would take (highlighted).</div>}
              {preview && <div className={preview.verdict.accept ? 'text-win' : 'text-loss'}>{preview.verdict.reason}</div>}
              {deal && !deal.valid && <div className="text-loss">{deal.problem}</div>}
              {notice && <div className="text-loss">{notice}</div>}
              {evalQ.isError && ready && <ErrorBox error={evalQ.error} />}
              {exec.isError && <ErrorBox error={exec.error} />}
              {counter.isError && <ErrorBox error={counter.error} />}
              {!deal && !preview && open && <div className="text-muted">Pick players and picks on each side. Up to {max} players and {maxPicks} picks per team; rosters are topped back up to 13 automatically.</div>}
            </div>
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            {preview ? (
              <>
                <button className="btn btn-accent" onClick={() => { setRawMoves(preview.moves); setPreview(null) }}><Check size={15} />Accept this change</button>
                <button className="btn" onClick={() => setPreview(null)}><X size={15} />Dismiss</button>
                <button className="btn btn-accent ml-auto" disabled={exec.isPending || !preview.verdict.accept} onClick={() => exec.mutate()}>Propose it now</button>
              </>
            ) : (
              <>
                <button className="btn btn-accent" disabled={!canPropose} onClick={() => exec.mutate()}>Propose trade</button>
                {refused && twoTeamNoPicks && <button className="btn" disabled={counter.isPending} onClick={() => counter.mutate()}><Sparkles size={15} />Ask what it would take</button>}
                {moves.length > 0 && <button className="btn" onClick={() => { setRawMoves([]); setNotice('') }}>Clear</button>}
              </>
            )}
          </div>
        </div>
      </LayoutGroup>
      <div id="transactions" className="scroll-mt-28"><Card title="Transactions">
        {txQ.data ? (
          txQ.data.transactions.length === 0 ? <div className="p-3 text-muted">No trades yet.</div> : (
            <ul className="max-h-[420px] overflow-y-auto"><AnimatePresence initial={false}>{txQ.data.transactions.map((t, i) => <TxItem key={`${t.year}-${t.day}-${i}-${t.moved[0]?.playerId}`} t={t} />)}</AnimatePresence></ul>
          )
        ) : <Spinner />}
      </Card></div>
    </Page>
  )
}
