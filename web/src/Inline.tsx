import { useSearchParams } from 'react-router-dom'
import { Loader2, Play } from 'lucide-react'
import { useFlow } from './flow'

export function InlineContinue({ big = false }: { big?: boolean }) {
  const { state, run, busy } = useFlow()
  if (!state) return null
  return (
    <button className={`btn btn-accent ${big ? 'px-5 py-3 text-base' : ''}`} disabled={busy} onClick={() => run(state.nextAction.id)}>
      {busy ? <Loader2 size={16} className="animate-spin" /> : <Play size={16} />}
      {state.nextAction.label}
    </button>
  )
}

export function useOpenPlayer() {
  const [, setSp] = useSearchParams()
  return (id: number) => setSp((p) => { p.set('player', String(id)); return p }, { replace: false })
}
