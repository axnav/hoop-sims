import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { api, type Change } from '../api'
import { useFlow } from '../flow'
import { InlineContinue } from '../Inline'
import { TxItem } from '../TxItem'
import { AnimatedNumber, Card, Page, PlayerLink, Q, TeamLink } from '../ui'

function Delta({ c, i }: { c: Change; i: number }) {
  const [v, setV] = useState(c.oldOvr)
  useEffect(() => {
    const t = setTimeout(() => setV(c.newOvr), 250 + i * 70)
    return () => clearTimeout(t)
  }, [c, i])
  const up = c.newOvr >= c.oldOvr
  return (
    <motion.tr initial={{ opacity: 0, x: -12 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: i * 0.05 }}>
      <td><PlayerLink id={c.playerId} name={c.name} /></td>
      <td className="text-muted"><TeamLink team={c.team} badge={false} short /></td>
      <td className="r">{c.age}</td>
      <td className="r text-muted">{c.oldOvr}</td>
      <td className={`r font-bold ${up ? 'text-win' : 'text-loss'}`}>
        <AnimatedNumber value={v} /> <span className="text-xs">({up ? '+' : ''}{c.newOvr - c.oldOvr})</span>
      </td>
    </motion.tr>
  )
}

function ChangeTable({ rows }: { rows: Change[] }) {
  return (
    <table className="tbl">
      <thead><tr><th>Player</th><th>Team</th><th className="r">Age</th><th className="r">Before</th><th className="r">After</th></tr></thead>
      <tbody>{rows.map((c, i) => <Delta key={c.playerId} c={c} i={i} />)}</tbody>
    </table>
  )
}

export default function Offseason() {
  const { state } = useFlow()
  const q = useQuery({ queryKey: ['offseason'], queryFn: api.offseason })
  return (
    <Page title="Offseason report" right={state?.phase === 'offseason' ? <InlineContinue /> : null}>
      <Q q={q} minH={400}>
        {(r) => {
          if (!r.available) return <div className="card p-8 text-center text-muted">No offseason in progress yet. Finish the playoffs first.</div>
          return (
            <div className="grid gap-4 xl:grid-cols-2">
              <Card title="Biggest risers"><ChangeTable rows={r.risers} /></Card>
              <Card title="Biggest fallers"><ChangeTable rows={r.fallers} /></Card>
              <Card title={`Retirements (${r.retirements.length})`}>
                <table className="tbl">
                  <thead><tr><th>Player</th><th>Team</th><th className="r">Age</th><th className="r">OVR</th></tr></thead>
                  <tbody>
                    {r.retirements.slice(0, 15).map((x) => (
                      <tr key={x.playerId}><td><PlayerLink id={x.playerId} name={x.name} /></td><td><TeamLink team={x.team} badge={false} short /></td><td className="r">{x.age}</td><td className="r">{x.ovr}</td></tr>
                    ))}
                  </tbody>
                </table>
                {r.retirements.length > 15 && <div className="px-2 pt-2 text-xs text-muted">and {r.retirements.length - 15} more.</div>}
              </Card>
              {r.trades && r.trades.length > 0 && (
                <Card title="Computer trades">
                  <ul>{r.trades.map((t, i) => <TxItem key={i} t={t} />)}</ul>
                </Card>
              )}
            </div>
          )
        }}
      </Q>
    </Page>
  )
}
