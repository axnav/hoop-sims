import { useQuery } from '@tanstack/react-query'
import { TrendingUp } from 'lucide-react'
import { api, type TakeAsset, type TakePackage, type UpgradeRow } from './api'
import { Chip, NameCell, PlayerLink, Q, TeamBadge, ovrColor, money } from './ui'

const assetText = (a: TakeAsset) => (a.kind === 'player' ? a.name : a.label)

function Assets({ title, list }: { title: string; list: TakeAsset[] }) {
  return (
    <div className="min-w-0">
      <div className="text-[10px] font-bold uppercase tracking-wide text-muted">{title}</div>
      <ul>
        {list.map((a, i) => a.kind === 'player' ? (
          <li key={i} className="flex min-w-0 items-center gap-2 py-0.5 text-[13px]">
            <span className="w-7 shrink-0 text-xs text-muted">{a.pos}</span>
            <span className="min-w-0 flex-1 truncate font-medium"><PlayerLink id={a.id} name={a.name} /></span>
            <span className="shrink-0 text-xs text-muted tabular-nums">{a.age}y</span>
            <b className="w-7 shrink-0 text-right tabular-nums" style={{ color: ovrColor(a.ovr) }}>{a.ovr}</b>
            <span className="w-14 shrink-0 text-right text-xs tabular-nums">{money(a.salary)}</span>
          </li>
        ) : (
          <li key={i} className="flex items-center gap-2 py-0.5 text-[13px]">
            <span className="w-7 shrink-0 text-xs text-muted">Pick</span>
            <span className="min-w-0 flex-1 truncate font-medium">{a.label}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function Head({ r }: { r: UpgradeRow }) {
  const p = r.player
  return (
    <div className="flex min-w-0 flex-wrap items-start gap-x-3 gap-y-1">
      <div className="min-w-0 flex-1 basis-48">
        <NameCell
          name={<span className="font-bold"><PlayerLink id={p.id} name={p.name} /></span>}
          chips={r.shopped ? <Chip tone="warn" title={r.shopReason}>Shopped</Chip> : undefined}
        />
        {r.shopped && r.shopReason && <div className="mt-0.5 hidden text-[12px] text-muted md:block">{r.shopReason}</div>}
      </div>
      <div className="flex items-center gap-2 text-[13px]">
        <TeamBadge team={r.team} size={18} /><span className="font-medium">{r.team.abbr}</span>
        <span className="text-xs text-muted">{p.pos}</span>
        <span className="text-xs text-muted tabular-nums">{p.age}y</span>
        <b className="tabular-nums" style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b>
        <span className="text-[12px] font-semibold text-win tabular-nums">+{r.gain}</span>
      </div>
    </div>
  )
}

// Better players at his position on computer teams, with a package the other team would accept.
export function UpgradeFinder({ playerId, bump, onLoad, onClose, disabled }: {
  playerId: number; bump?: unknown; disabled?: boolean; onClose: () => void
  onLoad: (teamId: number, pk: Pick<TakePackage, 'deal'>, labels: string[]) => void
}) {
  const q = useQuery({ queryKey: ['upgrades', playerId, bump], queryFn: () => api.findUpgrades(playerId), staleTime: 0 })
  return (
    <section className="card mb-3 border-accent/50 p-3" aria-label="Find an upgrade" id="upgrade-panel">
      <div className="mb-2 flex items-center gap-2">
        <h2 className="h2 flex min-w-0 flex-wrap items-center gap-x-1.5">
          <TrendingUp size={14} className="text-accent" />
          <span>Upgrades for {q.data?.player?.name ?? '...'}</span>
          {q.data?.player && <span className="text-xs font-normal text-muted">{q.data.player.pos}, <b style={{ color: ovrColor(q.data.player.ovr) }}>{q.data.player.ovr}</b> OVR</span>}
        </h2>
        <button className="btn ml-auto px-2 py-1 text-[12px]" onClick={onClose}>Close</button>
      </div>
      <Q q={q}>{(d) => {
        if (!d.possible) return <div className="text-[13px] font-semibold text-loss" role="status">{d.reason}</div>
        const all = d.upgrades ?? []
        if (all.length === 0) return <div className="text-[13px] text-muted">{d.reason || 'No better player at his position can be had right now.'}</div>
        const ok = all.filter((r) => r.possible && r.package)
        const out = all.filter((r) => !(r.possible && r.package))
        return (
          <>
            <div className="grid gap-3">
              {ok.map((r) => {
                const pk = r.package!
                return (
                  <div key={r.player.id} className="min-w-0 rounded-lg border border-line bg-panel2/40 p-2.5">
                    <Head r={r} />
                    <div className="mt-2 grid gap-2 md:grid-cols-2">
                      <Assets title="You give" list={pk.give ?? []} />
                      <Assets title="You get" list={pk.receive ?? []} />
                    </div>
                    {pk.note && <div className="mt-1.5 text-[12px] text-muted">{pk.note}</div>}
                    <button className="btn btn-accent mt-2" disabled={disabled}
                      onClick={() => onLoad(r.team.id, pk, [...(pk.give ?? []), ...(pk.receive ?? [])].map(assetText))}>Load this offer</button>
                  </div>
                )
              })}
            </div>
            {out.length > 0 && (
              <>
                <div className="mb-1 mt-3 text-[11px] font-bold uppercase tracking-wide text-muted">Out of reach for now</div>
                <div className="grid gap-2">
                  {out.map((r) => (
                    <div key={r.player.id} className="min-w-0 rounded-lg border border-line p-2.5 opacity-60">
                      <Head r={r} />
                      {r.reason && <div className="mt-1 text-[12px] text-muted">{r.reason}</div>}
                    </div>
                  ))}
                </div>
              </>
            )}
          </>
        )
      }}</Q>
    </section>
  )
}
