import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, LayoutGroup, motion } from 'motion/react'
import { Check, Lock, Sparkles, X } from 'lucide-react'
import { api, type Team, type TradeAssetRow, type TradeResult, type TradeVerdict } from '../api'
import { useFlow } from '../flow'
import { TxItem } from '../TxItem'
import { Card, ErrorBox, Page, PlayerLink, Q, Spinner, TeamBadge, ovrColor } from '../ui'

type Offer = { give: number[]; get: number[] }

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

function Tray({ title, rows, max, onRemove, hlIds }: { title: string; rows: TradeAssetRow[]; max: number; onRemove: (id: number) => void; hlIds: Set<number> }) {
  return (
    <div className="min-h-[132px] flex-1 rounded-lg border border-dashed border-line p-2">
      <div className="h2 mb-1 flex justify-between"><span>{title}</span><span>{rows.length}/{max}</span></div>
      <ul>
        {rows.map((p) => <Row key={p.id} p={p} maxV={Math.max(...rows.map((r) => r.value), 1)} disabled={false} hl={hlIds.has(p.id)} onPick={() => onRemove(p.id)} />)}
      </ul>
      {rows.length === 0 && <div className="py-4 text-center text-xs text-muted">Click players to add them</div>}
    </div>
  )
}

function Meter({ v }: { v: TradeVerdict }) {
  const need = v.theyGive * v.needed
  const fill = need > 0 ? Math.min(1, v.theyGet / need) : 0
  return (
    <div>
      <div className="h-3 overflow-hidden rounded-full bg-panel2">
        <motion.div className="h-full rounded-full" animate={{ width: `${fill * 100}%`, background: v.accept ? '#3ddc84' : '#ff5a67' }} transition={{ duration: 0.5 }} />
      </div>
      <div className="mt-1 flex justify-between text-[11px] text-muted">
        <span>They receive {v.theyGet.toFixed(1)}</span>
        <span>They want at least {need.toFixed(1)} (they give {v.theyGive.toFixed(1)})</span>
      </div>
    </div>
  )
}

function Summary({ r, onClose }: { r: TradeResult; onClose: () => void }) {
  const mine = r.state.user.team.id
  const sent = r.moved.filter((m) => m.from.id === mine)
  const got = r.moved.filter((m) => m.to.id === mine)
  const card = (m: (typeof sent)[number], dir: number) => (
    <motion.div key={m.playerId} className="card px-3 py-2 font-semibold" initial={{ x: dir * 160, opacity: 0.2 }} animate={{ x: 0, opacity: 1 }} transition={{ type: 'spring', stiffness: 160, damping: 18 }}>
      {m.name}
    </motion.div>
  )
  return (
    <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card mb-4 border-win/50 p-4">
      <div className="mb-3 flex items-center gap-2 text-lg font-bold text-win"><Check size={18} />Trade complete</div>
      <div className="grid gap-4 md:grid-cols-2">
        <div><div className="h2 mb-1">Sent to {r.teamA.id === mine ? r.teamB.abbr : r.teamA.abbr}</div><div className="space-y-1">{sent.map((m) => card(m, 1))}</div></div>
        <div><div className="h2 mb-1">Received</div><div className="space-y-1">{got.map((m) => card(m, -1))}</div></div>
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
  const [partner, setPartner] = useState(() => {
    const t = Number(sp.get('team'))
    return Number.isInteger(t) && sp.get('team') && t !== me ? t : me === 0 ? 1 : 0
  })
  const [offer, setOffer] = useState<Offer>(() => {
    const g = Number(sp.get('get'))
    return { give: [], get: sp.get('get') && Number.isInteger(g) ? [g] : [] }
  })
  const [preview, setPreview] = useState<(Offer & { verdict: TradeVerdict }) | null>(null)
  const [done, setDone] = useState<TradeResult | null>(null)
  const [notice, setNotice] = useState('')

  const mineQ = useQuery({ queryKey: ['trade-block', me, state?.phase, state?.day, done?.year, done?.day, done?.moved.length], queryFn: () => api.tradeBlock(me) })
  const theirQ = useQuery({ queryKey: ['trade-block', partner, state?.phase, state?.day, done?.moved.length], queryFn: () => api.tradeBlock(partner) })
  const txQ = useQuery({ queryKey: ['transactions'], queryFn: api.transactions })
  const open = status.data?.open ?? false
  const max = status.data?.maxPlayers ?? 3

  const shown = preview ?? offer
  const [deb, setDeb] = useState(shown)
  useEffect(() => {
    const t = setTimeout(() => setDeb(shown), 250)
    return () => clearTimeout(t)
  }, [shown.give.join(','), shown.get.join(','), partner]) // eslint-disable-line react-hooks/exhaustive-deps
  const ready = open && deb.give.length > 0 && deb.get.length > 0
  const evalQ = useQuery({
    queryKey: ['trade-eval', partner, deb.give, deb.get, state?.day],
    queryFn: () => api.tradeEvaluate({ team: partner, give: deb.give, get: deb.get }),
    enabled: ready,
    staleTime: 0,
  })
  const verdict = preview ? preview.verdict : ready ? evalQ.data : undefined
  const settled = deb.give.join() === shown.give.join() && deb.get.join() === shown.get.join()

  const counter = useMutation({
    mutationFn: () => api.tradeCounter({ team: partner, give: offer.give, get: offer.get }),
    onSuccess: (r) => {
      if (r.found) { setPreview({ give: r.give, get: r.get, verdict: r.verdict }); setNotice('') }
      else setNotice('They would not do a deal with those players, whatever else you added.')
    },
  })
  const exec = useMutation({
    mutationFn: () => api.tradeExecute({ team: partner, give: shown.give, get: shown.get }),
    onSuccess: (r) => {
      setDone(r); setOffer({ give: [], get: [] }); setPreview(null)
      qc.setQueryData(['state'], r.state)
      qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'state' })
    },
  })

  const edit = (side: 'give' | 'get', id: number) => {
    setPreview(null); setNotice(''); setDone(null); exec.reset()
    setOffer((o) => ({ ...o, [side]: o[side].includes(id) ? o[side].filter((x) => x !== id) : o[side].length < max ? [...o[side], id] : o[side] }))
  }
  const pickPartner = (id: number) => { setPartner(id); setOffer((o) => ({ ...o, get: [] })); setPreview(null); setNotice('') }

  const mineRows = mineQ.data?.players ?? [], theirRows = theirQ.data?.players ?? []
  const maxV = Math.max(1, ...mineRows.map((p) => p.value), ...theirRows.map((p) => p.value))
  const find = (rows: TradeAssetRow[], ids: number[]) => ids.map((id) => rows.find((r) => r.id === id)).filter((x): x is TradeAssetRow => !!x)
  const hlGive = new Set(preview ? preview.give.filter((x) => !offer.give.includes(x)) : [])
  const hlGet = new Set(preview ? preview.get.filter((x) => !offer.get.includes(x)) : [])
  const partnerTeam = teams.find((t) => t.id === partner)
  const canPropose = open && !!verdict?.valid && verdict.accept && settled && !exec.isPending
  const refused = !!verdict?.valid && !verdict.accept && !preview

  return (
    <Page title="Trades">
      {status.data && !open && (
        <div className="card mb-4 border-loss/50 p-4">
          <div className="font-bold text-loss">The trade window is closed</div>
          <div className="text-muted">{status.data.reason} {status.data.reopens}</div>
        </div>
      )}
      {done && <Summary r={done} onClose={() => setDone(null)} />}
      <div className="mb-3 flex flex-wrap gap-1.5">
        {teams.filter((t) => t.id !== me).map((t) => (
          <button key={t.id} onClick={() => pickPartner(t.id)} className={`rounded-md p-0.5 ${t.id === partner ? 'ring-2 ring-accent' : 'opacity-70 hover:opacity-100'}`} title={t.fullName}>
            <TeamBadge team={t} size={22} />
          </button>
        ))}
      </div>
      <LayoutGroup>
        <div className={`grid gap-4 lg:grid-cols-2 ${open ? '' : 'pointer-events-none'}`} style={open ? undefined : { opacity: 0.85 }}>
          <Card title="Your roster" right={<span className="text-[11px] text-muted">Pos · age · OVR · POT · value</span>}>
            <Q q={mineQ}>{() => <ul>{mineRows.filter((p) => !offer.give.includes(p.id) && !(preview?.give.includes(p.id))).map((p) => <Row key={p.id} p={p} maxV={maxV} disabled={!open} onPick={() => edit('give', p.id)} />)}</ul>}</Q>
          </Card>
          <Card title={partnerTeam ? partnerTeam.fullName : 'Partner'} right={<span className="flex items-center gap-1 text-[11px] text-muted"><Lock size={11} />core players cost a lot</span>}>
            <Q q={theirQ}>{() => <ul>{theirRows.filter((p) => !offer.get.includes(p.id) && !(preview?.get.includes(p.id))).map((p) => <Row key={p.id} p={p} maxV={maxV} disabled={!open} onPick={() => edit('get', p.id)} />)}</ul>}</Q>
          </Card>
        </div>
        <div className="card my-4 p-3">
          <div className="flex flex-col gap-3 md:flex-row">
            <Tray title="You send" rows={find(mineRows, shown.give)} max={max} hlIds={hlGive} onRemove={(id) => open && !preview && edit('give', id)} />
            <Tray title="You receive" rows={find(theirRows, shown.get)} max={max} hlIds={hlGet} onRemove={(id) => open && !preview && edit('get', id)} />
          </div>
          <div className="mt-3 min-h-[88px]">
            {verdict && verdict.valid && <Meter v={verdict} />}
            <div className="mt-2 text-[13px]">
              {preview && <div className="mb-1 flex items-center gap-1 font-semibold text-accent"><Sparkles size={14} />This is what it would take (highlighted).</div>}
              {verdict && !verdict.valid && <div className="text-loss">{verdict.problem}</div>}
              {verdict && verdict.valid && <div className={verdict.accept ? 'text-win' : 'text-loss'}>{verdict.reason}</div>}
              {notice && <div className="text-loss">{notice}</div>}
              {evalQ.isError && ready && <ErrorBox error={evalQ.error} />}
              {exec.isError && <ErrorBox error={exec.error} />}
              {counter.isError && <ErrorBox error={counter.error} />}
              {!verdict && open && <div className="text-muted">Pick players on both sides. Up to {max} each; rosters are topped back up to 13 automatically.</div>}
            </div>
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            {preview ? (
              <>
                <button className="btn btn-accent" onClick={() => { setOffer({ give: preview.give, get: preview.get }); setPreview(null) }}><Check size={15} />Accept this change</button>
                <button className="btn" onClick={() => setPreview(null)}><X size={15} />Dismiss</button>
              </>
            ) : (
              <>
                <button className="btn btn-accent" disabled={!canPropose} onClick={() => exec.mutate()}>Propose trade</button>
                {refused && <button className="btn" disabled={counter.isPending} onClick={() => counter.mutate()}><Sparkles size={15} />Ask what it would take</button>}
                {(offer.give.length > 0 || offer.get.length > 0) && <button className="btn" onClick={() => { setOffer({ give: [], get: [] }); setNotice('') }}>Clear</button>}
              </>
            )}
            {preview && <button className="btn btn-accent ml-auto" disabled={exec.isPending} onClick={() => exec.mutate()}>Propose it now</button>}
          </div>
        </div>
      </LayoutGroup>
      <Card title="Transactions">
        {txQ.data ? (
          txQ.data.transactions.length === 0 ? <div className="p-3 text-muted">No trades yet.</div> : (
            <ul className="max-h-[420px] overflow-y-auto"><AnimatePresence initial={false}>{txQ.data.transactions.map((t, i) => <TxItem key={`${t.year}-${t.day}-${i}-${t.moved[0]?.playerId}`} t={t} />)}</AnimatePresence></ul>
          )
        ) : <Spinner />}
      </Card>
    </Page>
  )
}
