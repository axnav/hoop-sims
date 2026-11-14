import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { FolderOpen, Loader2, Play } from 'lucide-react'
import { api, type Meta } from '../api'
import { ErrorBox, TeamBadge } from '../ui'

export default function Start({ meta }: { meta: Meta }) {
  const qc = useQueryClient()
  const [mode, setMode] = useState<'fictional' | 'real'>('fictional')
  const [seed, setSeed] = useState('1')
  const [team, setTeam] = useState(0)
  const teams = mode === 'real' ? meta.realTeams : meta.fictionalTeams
  const done = (s: Awaited<ReturnType<typeof api.state>>) => {
    qc.setQueryData(['state'], s)
    qc.invalidateQueries({ queryKey: ['meta'] })
  }
  const create = useMutation({
    mutationFn: () => api.newGame({ mode, seed: Math.max(1, Number(seed) || 1), team }),
    onSuccess: done,
  })
  const load = useMutation({ mutationFn: api.load, onSuccess: done })
  const err = create.error ?? load.error
  const sel = teams[team]
  return (
    <div className="mx-auto max-w-5xl px-5 py-10">
      <h1 className="text-5xl font-black tracking-tight">HOOP<span className="text-accent">SIM</span></h1>
      <p className="mt-1 text-muted">Pick a franchise and run it.</p>
      <div className="mt-6 flex flex-wrap items-center gap-3">
        <div className="flex overflow-hidden rounded-lg border border-line">
          <button className={`px-4 py-2 font-semibold ${mode === 'fictional' ? 'bg-accent text-black' : 'bg-panel'}`} onClick={() => { setMode('fictional'); setTeam(0) }}>Fictional league</button>
          {meta.realAvailable && (
            <button className={`px-4 py-2 font-semibold ${mode === 'real' ? 'bg-accent text-black' : 'bg-panel'}`} onClick={() => { setMode('real'); setTeam(0) }}>Real 2010-11 league</button>
          )}
        </div>
        <label className="flex items-center gap-2 text-muted">Seed
          <input className="w-24 rounded-md border border-line bg-panel px-2 py-1.5 text-ink" value={seed} onChange={(e) => setSeed(e.target.value.replace(/\D/g, ''))} />
        </label>
        {meta.saveExists && (
          <button className="btn ml-auto" disabled={load.isPending} onClick={() => load.mutate()}>
            {load.isPending ? <Loader2 size={15} className="animate-spin" /> : <FolderOpen size={15} />} Load saved game
          </button>
        )}
      </div>
      {err && <div className="mt-3"><ErrorBox error={err} /></div>}
      {[0, 1].map((conf) => (
        <div key={conf} className="mt-6">
          <div className="h2 mb-2">{meta.conferences[conf]}</div>
          <div className="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-5">
            {teams.filter((t) => t.conf === conf).map((t) => (
              <motion.button
                key={`${mode}-${t.id}`} whileHover={{ y: -2 }} whileTap={{ scale: 0.98 }}
                onClick={() => setTeam(t.id)}
                className={`flex items-center gap-3 rounded-xl border p-3 text-left ${team === t.id ? 'border-accent bg-panel2' : 'border-line bg-panel'}`}
              >
                <TeamBadge team={t} size={30} />
                <span className="leading-tight"><span className="block text-[11px] text-muted">{t.city}</span><span className="font-bold">{t.name}</span></span>
              </motion.button>
            ))}
          </div>
        </div>
      ))}
      <div className="sticky bottom-4 mt-8 flex justify-end">
        <button className="btn btn-accent px-6 py-3 text-base shadow-xl" disabled={create.isPending || !sel} onClick={() => create.mutate()}>
          {create.isPending ? <Loader2 size={16} className="animate-spin" /> : <Play size={16} />} Start as {sel?.fullName}
        </button>
      </div>
    </div>
  )
}
