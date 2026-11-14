import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { Pause, Play } from 'lucide-react'
import { api, type Pick } from '../api'
import { useFlow } from '../flow'
import { InlineContinue } from '../Inline'
import { Card, DataTable, ErrorBox, Page, PlayerLink, Q, TeamBadge, ovrColor, type Col } from '../ui'

type Prospect = { id: number; name: string; age: number; pos: string; ovr: number; pot: number }

function PickCell({ p, me, current }: { p: Pick; me: boolean; current: boolean }) {
  return (
    <div className={`flex items-center gap-2 rounded px-2 py-1 text-[13px] ${current ? 'bg-accent/20 ring-1 ring-accent' : me ? 'bg-white/10' : ''}`}>
      <span className="w-6 text-muted tabular-nums">{p.pick}</span>
      <TeamBadge team={p.team} size={16} />
      <AnimatePresence mode="wait" initial={false}>
        {p.made ? (
          <motion.span key="m" initial={{ opacity: 0, x: 12 }} animate={{ opacity: 1, x: 0 }} className="flex-1 truncate">
            <PlayerLink id={p.player!.id} name={p.player!.name} /> <span className="text-muted tabular-nums">{p.player!.ovr}/{p.player!.pot}</span>
          </motion.span>
        ) : (
          <motion.span key="u" className="flex-1 text-muted">{current ? 'On the clock' : '-'}</motion.span>
        )}
      </AnimatePresence>
    </div>
  )
}

export default function Draft() {
  const { state, run, busy } = useFlow()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['draft'], queryFn: api.draft })
  const [running, setRunning] = useState(true)
  const [err, setErr] = useState('')
  const d = q.data && q.data.active ? q.data : null
  const made = d?.made

  useEffect(() => {
    if (!d || !d.started || d.done || d.userOnClock || !running) return
    const t = setTimeout(async () => {
      try {
        const r = await api.draftStep(1)
        qc.setQueryData(['state'], r.state)
        await qc.invalidateQueries({ queryKey: ['draft'] })
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
        setRunning(false)
      }
    }, 450)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [made, d?.userOnClock, d?.started, running])

  const pick = async (id: number) => {
    setErr('')
    try {
      const r = await api.draftPick(id)
      qc.setQueryData(['state'], r.state)
      await qc.invalidateQueries({ queryKey: ['draft'] })
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  const cols: Col<Prospect>[] = [
    { accessorKey: 'name', header: 'Prospect', cell: (i) => <PlayerLink id={i.row.original.id} name={i.getValue()} />, sortDescFirst: false },
    { accessorKey: 'pos', header: 'Pos', sortDescFirst: false },
    { accessorKey: 'age', header: 'Age', num: true },
    { accessorKey: 'ovr', header: 'OVR', num: true, cell: (i) => <b style={{ color: ovrColor(i.getValue()) }}>{i.getValue()}</b> },
    { accessorKey: 'pot', header: 'POT', num: true },
    {
      id: 'pick', header: '', enableSorting: false,
      cell: (i) => d?.userOnClock ? <button className="btn btn-accent py-0.5" onClick={() => pick(i.row.original.id)}>Pick</button> : null,
    },
  ]

  return (
    <Page title="Draft" right={d && !d.done && d.started ? (
      <div className="flex items-center gap-2">
        {!d.userOnClock && <button className="btn" onClick={() => setRunning((r) => !r)}>{running ? <Pause size={15} /> : <Play size={15} />}{running ? 'Pause' : 'Resume'}</button>}
        <button className="btn" disabled={busy} onClick={() => run('draft_auto')}>Auto-draft the rest</button>
      </div>
    ) : null}>
      {err && <ErrorBox error={err} />}
      <Q q={q} minH={500}>
        {(data) => {
          if (!data.active) return <div className="card p-8 text-center text-muted">There is no draft right now.</div>
          const cur = data.picks.find((p) => !p.made)?.pick
          const me = data.userTeam
          return (
            <>
              {!data.started && <div className="card mb-4 p-4">Look at the <Link to="/offseason" className="link text-accent">offseason report</Link> first, then go to the draft.</div>}
              {data.started && data.userOnClock && (
                <motion.div initial={{ opacity: 0, y: -8 }} animate={{ opacity: 1, y: 0 }} className="card mb-4 border-accent p-4 text-lg font-bold">
                  You are on the clock: pick #{cur}. Choose a prospect below.
                </motion.div>
              )}
              {data.done && (
                <div className="card mb-4 flex items-center justify-between p-4">
                  <span className="text-lg font-bold">Draft complete.</span>
                  {state?.phase === 'draft_done' && <InlineContinue big />}
                </div>
              )}
              <div className="grid gap-4 xl:grid-cols-[360px_1fr]">
                <Card title={`Picks (${data.made}/${data.total})`}>
                  <div className="max-h-[640px] overflow-y-auto pr-1">
                    {data.picks.map((p) => <PickCell key={p.pick} p={p} me={p.team.id === me} current={p.pick === cur && !data.done} />)}
                  </div>
                </Card>
                <Card title={`Available prospects (${data.prospects.length})`}>
                  <div className="max-h-[640px] overflow-y-auto">
                    <DataTable data={data.prospects} columns={cols} initialSort={[{ id: 'ovr', desc: true }]} />
                  </div>
                </Card>
              </div>
            </>
          )
        }}
      </Q>
    </Page>
  )
}
