import { useId } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Options } from '../api'
import { Card, ErrorBox, Page, Q } from '../ui'

function Switch({ label, note, checked, disabled, onChange }: { label: string; note?: string; checked: boolean; disabled?: boolean; onChange: (v: boolean) => void }) {
  const id = useId()
  return (
    <div className="flex items-start gap-3 border-b border-line/50 px-2 py-3 last:border-b-0">
      <button
        id={id} type="button" role="switch" aria-checked={checked} aria-labelledby={`${id}-l`} disabled={disabled}
        onClick={() => onChange(!checked)}
        className={`relative mt-0.5 h-6 w-11 shrink-0 rounded-full transition-colors disabled:opacity-60 ${checked ? 'bg-accent' : 'bg-line'}`}
      >
        <span className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${checked ? 'left-[22px]' : 'left-0.5'}`} />
      </button>
      <div className="min-w-0">
        <label id={`${id}-l`} htmlFor={id} className="cursor-pointer font-semibold">{label}</label>
        {note && <div className="text-xs text-muted">{note}</div>}
      </div>
    </div>
  )
}

export default function Settings() {
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['options'], queryFn: api.options })
  const save = useMutation({
    mutationFn: (o: Partial<Options>) => api.setOptions(o),
    onSuccess: (o) => {
      qc.setQueryData(['options'], o)
      qc.invalidateQueries({ queryKey: ['player'] })
    },
  })
  const set = (o: Partial<Options>) => save.mutate(o)
  return (
    <Page title="Settings">
      <div className="max-w-2xl">
        <Q q={q} minH={200}>{(o) => (
          <Card>
            <Switch label="Pause the sim the day before the trade deadline" checked={!o.noDeadlineStop} disabled={save.isPending} onChange={(v) => set({ noDeadlineStop: !v })} />
            <Switch label="Pause the sim when a team makes you an offer" checked={!o.noOfferStop} disabled={save.isPending} onChange={(v) => set({ noOfferStop: !v })} />
            <Switch label="Injuries" note="Players who are already injured still heal when this is off." checked={!o.noInjuries} disabled={save.isPending} onChange={(v) => set({ noInjuries: !v })} />
            <Switch label="In-season progression" note="Ratings also move in small steps during the season. Off means they only change in the offseason." checked={!o.noInSeasonDev} disabled={save.isPending} onChange={(v) => set({ noInSeasonDev: !v })} />
            <Switch label="Commissioner mode" note="Allows editing any player's ratings, age and potential." checked={o.commissioner} disabled={save.isPending} onChange={(v) => set({ commissioner: v })} />
          </Card>
        )}</Q>
        {save.isError && <div className="mt-2"><ErrorBox error={save.error} /></div>}
      </div>
    </Page>
  )
}
