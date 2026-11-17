import { Fragment } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, type Award, type SeasonAwards } from '../api'
import { Card, Page, PlayerLink, Q, TeamBadge, TeamLink, f1 } from '../ui'

const A = ({ a }: { a: Award | undefined }) => (a ? <span><PlayerLink id={a.playerId} name={a.name} /> <span className="text-muted">{a.team.abbr}</span></span> : <span className="text-muted">-</span>)

const Names = ({ list }: { list: Award[] }) => (
  <div className="flex flex-wrap gap-x-3 gap-y-1">
    {list.filter((a) => a).map((a) => (
      <span key={a!.playerId} className="inline-flex items-center gap-1"><TeamBadge team={a!.team} size={16} /><PlayerLink id={a!.playerId} name={a!.name} /></span>
    ))}
  </div>
)

function AwardList({ aw }: { aw: SeasonAwards }) {
  const rows: [string, Award | undefined][] = [['Rookie', aw.roy], ['Defensive Player', aw.dpoy], ['Sixth Man', aw.sixthMan], ['Most Improved', aw.mip]]
  const c = aw.coach
  return (
    <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2 lg:grid-cols-3">
      {rows.map(([l, a]) => <div key={l} className="min-w-0"><span className="text-muted">{l}: </span><A a={a} /></div>)}
      <div className="min-w-0"><span className="text-muted">Coach: </span>{c ? <span>{c.name} <span className="text-muted">{c.team.abbr} {c.w}-{c.l}</span></span> : <span className="text-muted">-</span>}</div>
    </div>
  )
}

function Teams({ aw }: { aw: SeasonAwards }) {
  const all = (aw.allLeague ?? []).filter((t) => t && t.length > 0)
  const rookie = (aw.allRookie ?? []).filter((a) => a)
  if (all.length === 0 && rookie.length === 0) return null
  const names = ['First', 'Second', 'Third']
  return (
    <details className="mt-2">
      <summary className="cursor-pointer text-muted hover:text-ink">All-League teams</summary>
      <div className="mt-1.5 space-y-1.5">
        {all.map((t, i) => <div key={i}><div className="h2">{names[i] ?? `Team ${i + 1}`} team</div><Names list={t} /></div>)}
        {rookie.length > 0 && <div><div className="h2">All-Rookie team</div><Names list={rookie} /></div>}
      </div>
    </details>
  )
}

const hasAwards = (aw?: SeasonAwards | null) => !!aw && !!(aw.roy || aw.dpoy || aw.sixthMan || aw.mip || aw.coach || (aw.allLeague?.length ?? 0) > 0 || (aw.allRookie?.length ?? 0) > 0)

export default function History() {
  const q = useQuery({ queryKey: ['history'], queryFn: api.history })
  return (
    <Page title="League history">
      <Q q={q} minH={300}>
        {(d) => (
          <>
            {d.current && (
              <Card title={`This season's awards (${d.current.year})`} className="mb-4">
                <div className="space-y-1 p-3">
                  <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2 lg:grid-cols-3">
                    <div><span className="text-muted">MVP: </span><A a={d.current.mvp ?? undefined} /></div>
                    <div><span className="text-muted">Scoring leader: </span><A a={d.current.scoringLeader ?? undefined} />{d.current.scoringLeader && <span className="text-muted tabular-nums"> {f1(d.current.scoringLeader.ppg)}</span>}</div>
                  </div>
                  {d.current.awards && <AwardList aw={d.current.awards} />}
                  {d.current.awards && <Teams aw={d.current.awards} />}
                </div>
              </Card>
            )}
            {d.seasons.length === 0 ? (
              <div className="card p-8 text-center text-muted">No finished seasons yet.</div>
            ) : (
              <Card>
                <table className="tbl">
                  <thead><tr><th>Year</th><th>Champion</th><th>Finals</th><th>MVP</th><th>Finals MVP</th><th>Scoring leader</th></tr></thead>
                  <tbody>
                    {d.seasons.map((r) => (
                      <Fragment key={r.year}>
                        <tr>
                          <td className="font-bold">{r.year}</td><td><TeamLink team={r.champion} /></td>
                          <td className="text-muted tabular-nums">{r.finals ? `over ${r.finals.loser.name}, ${r.finals.winnerWins}-${r.finals.loserWins}` : '-'}</td>
                          <td><A a={r.mvp ?? undefined} /></td><td><A a={r.finalsMvp ?? undefined} /></td>
                          <td><A a={r.scoringLeader ?? undefined} />{r.scoringLeader && <span className="text-muted tabular-nums"> {f1(r.scoringLeader.ppg)}</span>}</td>
                        </tr>
                        {hasAwards(r.awards) && (
                          <tr>
                            <td />
                            <td colSpan={5} className="!whitespace-normal text-[13px]"><AwardList aw={r.awards!} /><Teams aw={r.awards!} /></td>
                          </tr>
                        )}
                      </Fragment>
                    ))}
                  </tbody>
                </table>
              </Card>
            )}
          </>
        )}
      </Q>
    </Page>
  )
}
