import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type GameState, type Phase } from './api'
import { SimOverlay, type SimPlan } from './SimOverlay'

export const homeRoute = (p: Phase) =>
  ({ regular: '/', regular_done: '/', playoffs: '/playoffs', playoffs_done: '/', offseason: '/offseason', draft: '/draft', draft_done: '/' })[p]

type Flow = {
  state: GameState | undefined
  error: unknown
  busy: boolean
  run: (id: string) => Promise<void>
  actionError: string
}
const Ctx = createContext<Flow | null>(null)
export const useFlow = () => useContext(Ctx)!

export function FlowProvider({ children }: { children: ReactNode }) {
  const nav = useNavigate()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['state'], queryFn: api.state })
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState('')
  const [plan, setPlan] = useState<SimPlan | null>(null)

  const apply = useCallback(
    (prev: Phase | undefined, ns: GameState) => {
      qc.setQueryData(['state'], ns)
      qc.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'state' })
      if (prev && ns.phase !== prev) nav(homeRoute(ns.phase))
    },
    [qc, nav],
  )

  const run = useCallback(
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

  const closeSim = () => {
    const prev = qc.getQueryData<GameState>(['state'])
    setPlan(null)
    qc.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'state' })
    if (prev && prev.phase !== 'regular') nav(homeRoute(prev.phase))
  }

  return (
    <Ctx.Provider value={{ state: q.data, error: q.error, busy: busy || !!plan, run, actionError }}>
      {children}
      {plan && <SimOverlay plan={plan} onClose={closeSim} />}
    </Ctx.Provider>
  )
}
