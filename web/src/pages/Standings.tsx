import { useQuery } from '@tanstack/react-query'
import { LayoutGroup, motion } from 'motion/react'
import { api } from '../api'
import { Card, Page, Q, TeamLink, f1, pct3 } from '../ui'

export default function Standings() {
  const q = useQuery({ queryKey: ['standings'], queryFn: api.standings })
  return (
    <Page title="Standings">
      <Q q={q} minH={500}>
        {(d) => (
          <div className="grid gap-4 xl:grid-cols-2">
            {d.conferences.map((c) => (
              <Card key={c.conf} title={`${c.name} conference`}>
                <LayoutGroup id={`c${c.conf}`}>
                  <table className="tbl">
                    <thead><tr><th>#</th><th>Team</th><th className="r">W</th><th className="r">L</th><th className="r">PCT</th><th className="r">GB</th><th className="r">DIFF</th></tr></thead>
                    <tbody>
                      {c.rows.map((r) => (
                        <motion.tr
                          key={r.team.id} layout="position" transition={{ type: 'spring', stiffness: 420, damping: 38 }}
                          className={r.team.id === d.userTeam ? 'me' : ''}
                          style={r.rank === d.playoffLine + 1 ? { boxShadow: 'inset 0 2px 0 var(--color-accent)' } : undefined}
                        >
                          <td className="text-muted">{r.rank}</td>
                          <td><TeamLink team={r.team} /></td>
                          <td className="r">{r.w}</td><td className="r">{r.l}</td><td className="r">{pct3(r.pct)}</td>
                          <td className="r">{r.gb === 0 ? '-' : f1(r.gb)}</td>
                          <td className={`r ${r.pd > 0 ? 'text-win' : r.pd < 0 ? 'text-loss' : ''}`}>{r.pd > 0 ? '+' : ''}{r.pd}</td>
                        </motion.tr>
                      ))}
                    </tbody>
                  </table>
                </LayoutGroup>
              </Card>
            ))}
          </div>
        )}
      </Q>
      <p className="mt-3 text-xs text-muted">The orange line marks the playoff cut (top 8 in each conference).</p>
    </Page>
  )
}
