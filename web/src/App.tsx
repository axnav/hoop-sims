import { useEffect } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from './api'
import { FlowProvider, useFlow } from './flow'
import { TopBar } from './TopBar'
import { PlayerDrawer } from './PlayerDrawer'
import { ErrorBox, Spinner } from './ui'
import Start from './pages/Start'
import Dashboard from './pages/Dashboard'
import TeamPage from './pages/TeamPage'
import Standings from './pages/Standings'
import Teams from './pages/Teams'
import Players from './pages/Players'
import Leaders from './pages/Leaders'
import Schedule from './pages/Schedule'
import BoxScore from './pages/BoxScore'
import Playoffs from './pages/Playoffs'
import GameViewer from './pages/GameViewer'
import Offseason from './pages/Offseason'
import Draft from './pages/Draft'
import FreeAgency from './pages/FreeAgency'
import History from './pages/History'
import Lineup from './pages/Lineup'
import Coach from './pages/Coach'
import Trade from './pages/Trade'
import Settings from './pages/Settings'

function Shell() {
  const { state, error, run, busy, actionError } = useFlow()
  useEffect(() => {
    const root = document.documentElement
    if (state) {
      root.style.setProperty('--team', state.user.team.primary)
      root.style.setProperty('--team2', state.user.team.secondary)
    }
  }, [state])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (e.key !== ' ' && e.key !== 'Enter') return
      if (e.ctrlKey || e.metaKey || e.altKey || !state || busy) return
      const t = e.target as HTMLElement
      if (t.closest('input, textarea, select, button, a, [contenteditable="true"]')) return
      e.preventDefault()
      run(state.nextAction.id)
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [state, busy, run])

  if (!state) return error ? <ErrorBox error={error} /> : <Spinner />
  return (
    <>
      <TopBar />
      {actionError && <div className="mx-auto max-w-[1400px] px-3 pt-3 min-[1000px]:px-5"><ErrorBox error={actionError} /></div>}
      <main>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/roster" element={<Navigate to={`/team/${state.user.team.id}`} replace />} />
          <Route path="/lineup" element={<Lineup />} />
          <Route path="/coach" element={<Coach />} />
          <Route path="/coach/:teamId" element={<Coach />} />
          <Route path="/trade" element={<Trade />} />
          <Route path="/team/:id" element={<TeamPage />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/standings" element={<Standings />} />
          <Route path="/teams" element={<Teams />} />
          <Route path="/players" element={<Players />} />
          <Route path="/leaders" element={<Leaders />} />
          <Route path="/schedule" element={<Schedule />} />
          <Route path="/schedule/:id" element={<Schedule />} />
          <Route path="/box/:id" element={<BoxScore />} />
          <Route path="/playoffs" element={<Playoffs />} />
          <Route path="/game/:id" element={<GameViewer />} />
          <Route path="/offseason" element={<Offseason />} />
          <Route path="/draft" element={<Draft />} />
          <Route path="/free-agency" element={<FreeAgency />} />
          <Route path="/history" element={<History />} />
          <Route path="*" element={<Dashboard />} />
        </Routes>
      </main>
      <PlayerDrawer />
    </>
  )
}

export default function App() {
  const meta = useQuery({ queryKey: ['meta'], queryFn: api.meta })
  if (meta.isError) return <ErrorBox error={meta.error} />
  if (!meta.data) return <Spinner />
  if (!meta.data.loaded) return <Start meta={meta.data} />
  return (
    <FlowProvider>
      <Shell />
    </FlowProvider>
  )
}
