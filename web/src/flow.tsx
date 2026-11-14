import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type FASigning, type GameState, type Phase } from './api'
import { ConfirmDialog } from './ui'
import { SimOverlay, type SimPlan } from './SimOverlay'

export const homeRoute = (p: Phase) =>
  ({ regular: '/', regular_done: '/', playoffs: '/playoffs', playoffs_done: '/', offseason: '/offseason', draft: '/draft', draft_done: '/', free_agency: '/free-agency' })[p]

type Flow = {
  state: GameState | undefined
  error: unknown
  busy: boolean
  run: (id: string) => Promise<void>
  actionError: string
  lastSignings: FASigning[]
}
const Ctx = createContext<Flow | null>(null)
export const useFlow = () => useContext(Ctx)!

export function FlowProvider({ children }: { children: ReactNode }) {
  const nav = useNavigate()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['state'], queryFn: api.state })
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState('')
  const [lastSignings, setLastSignings] = useState<FASigning[]>([])
  const [ask, setAsk] = useState<{ message: string; id: string } | null>(null)
  const [plan, setPlan] = useState<SimPlan | null>(null)

  const apply = useCallback(
    (prev: Phase | undefined, ns: GameState) => {
      qc.setQueryData(['state'], ns)
      qc.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'state' })
      if (prev && ns.phase !== prev) nav(homeRoute(ns.phase))
    },
    [qc, nav],
  )

  const exec = useCallback(
    async (id: string) => {
      const prev = qc.getQueryData<GameState>(['state'])?.phase
      if (busy || plan) return
      setActionError('')
      setBusy(true)
      try {
        switch (id) {
          case 'sim_day': apply(prev, (await api.sim({ days: 1 })).state); break
          case 'sim_week': setPlan({ days: 7, toEnd: false }); break
          case 'sim_to_end': setPlan({ days: 0, toEnd: true }); break
          case 'start_playoffs': apply(prev, (await api.playoffAction('start')).state); break
          case 'playoff_day': apply(prev, (await api.playoffAction('day')).state); break
          case 'playoff_series': apply(prev, (await api.playoffAction('series')).state); break
          case 'playoff_round': apply(prev, (await api.playoffAction('round')).state); break
          case 'playoff_all': apply(prev, (await api.playoffAction('all')).state); break
          case 'playoff_watch': {
            const r = await api.playoffAction('watch')
            apply(prev, r.state)
            nav(`/game/${r.boxId}`)
            break
          }
          case 'begin_offseason': apply(prev, await api.offseasonBegin()); break
          case 'go_draft': apply(prev, await api.offseasonToDraft()); nav('/draft'); break
          case 'make_pick': case 'draft_run': nav('/draft'); break
          case 'draft_auto': apply(prev, await api.draftAuto()); break
          case 'begin_free_agency': {
            const r = await api.faBegin()
            setLastSignings([])
            apply(prev, r.state)
            qc.setQueryData(['freeagents'], r.freeAgency)
            nav('/free-agency')
            break
          }
          case 'fa_day': {
            const r = await api.faDay()
            setLastSignings(r.signings)
            apply(prev, r.state)
            qc.setQueryData(['freeagents'], r.freeAgency)
            break
          }
          case 'start_season': apply(prev, await api.offseasonFinish()); break
        }
      } catch (e) {
        setActionError(e instanceof Error ? e.message : String(e))
        qc.invalidateQueries()
      } finally {
        setBusy(false)
      }
    },
    [apply, busy, plan, nav, qc],
  )

  // Some actions ask first; the dialog's OK then runs exec.
  const run = useCallback(
    async (id: string) => {
      if (busy || plan) return
      const st = qc.getQueryData<GameState>(['state'])
      if (id === 'begin_free_agency' && st && st.expiring > 0) {
        setAsk({ id, message: `${st.expiring} of your players have not been re-signed and will become free agents. Start free agency?` })
        return
      }
      if (id === 'start_season' && st?.phase === 'free_agency') {
        try {
          const fa = await qc.fetchQuery({ queryKey: ['freeagents'], queryFn: api.freeAgents, staleTime: 0 })
          const open = fa.rosterSize - fa.rosterCount
          if (open > 0) {
            setAsk({ id, message: `You have ${open} open roster ${open === 1 ? 'spot' : 'spots'}. They will be filled with minimum-salary players nobody signed. Start the new season?` })
            return
          }
        } catch { /* fall through and let the server decide */ }
      }
      await exec(id)
    },
    [busy, plan, qc, exec],
  )

  const closeSim = () => {
    const prev = qc.getQueryData<GameState>(['state'])
    setPlan(null)
    qc.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'state' })
    if (prev && prev.phase !== 'regular') nav(homeRoute(prev.phase))
  }

  return (
    <Ctx.Provider value={{ state: q.data, error: q.error, busy: busy || !!plan, run, actionError, lastSignings }}>
      {children}
      {ask && (
        <ConfirmDialog
          message={ask.message}
          onCancel={() => setAsk(null)}
          onConfirm={() => { const id = ask.id; setAsk(null); void exec(id) }}
        />
      )}
      {plan && <SimOverlay plan={plan} onClose={closeSim} />}
    </Ctx.Provider>
  )
}
