import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Search, X } from 'lucide-react'
import { api } from './api'
import { useOpenPlayer } from './Inline'
import { Avatar, ovrColor } from './ui'

// Top-bar name search: a short dropdown of matches; Enter or "See all" opens the Players page.
export function PlayerSearch() {
  const nav = useNavigate()
  const open = useOpenPlayer()
  const [text, setText] = useState('')
  const [term, setTerm] = useState('')
  const [show, setShow] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const t = setTimeout(() => setTerm(text.trim()), 200)
    return () => clearTimeout(t)
  }, [text])
  useEffect(() => {
    const h = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) { setShow(false); setMobileOpen(false) } }
    window.addEventListener('mousedown', h)
    return () => window.removeEventListener('mousedown', h)
  }, [])
  useEffect(() => { if (mobileOpen) input.current?.focus() }, [mobileOpen])

  const res = useQuery({
    queryKey: ['player-search', term], queryFn: () => api.players(`q=${encodeURIComponent(term)}&limit=8`),
    enabled: term.length > 0, staleTime: 15000,
  })
  const seeAll = () => {
    if (!text.trim()) return
    setShow(false); setMobileOpen(false)
    nav(`/players?q=${encodeURIComponent(text.trim())}`)
  }
  const pick = (id: number) => { setShow(false); setMobileOpen(false); setText(''); setTerm(''); open(id) }
  const rows = res.data?.players ?? []

  return (
    <div ref={ref} className={mobileOpen ? 'max-[699px]:absolute max-[699px]:inset-x-0 max-[699px]:top-0 max-[699px]:z-40 max-[699px]:bg-[#090c12] max-[699px]:p-2' : 'relative'}>
      <div className={`min-[700px]:hidden ${mobileOpen ? 'hidden' : ''}`}>
        <button type="button" className="btn" aria-label="Search players" onClick={() => setMobileOpen(true)}><Search size={15} /></button>
      </div>
      <div className={`relative ${mobileOpen ? '' : 'max-[699px]:hidden'}`}>
        <Search size={14} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted" />
        <input
          ref={input} type="search" aria-label="Search players" placeholder="Search players" value={text}
          onChange={(e) => { setText(e.target.value); setShow(true) }} onFocus={() => setShow(true)}
          onKeyDown={(e) => { if (e.key === 'Enter') seeAll(); if (e.key === 'Escape') { setShow(false); setMobileOpen(false) } }}
          className="w-full rounded-md border border-line bg-panel py-1.5 pl-8 pr-7 text-[13px] text-ink placeholder:text-muted min-[700px]:w-40 min-[1000px]:w-52"
        />
        {mobileOpen && <button type="button" aria-label="Close search" className="absolute right-1.5 top-1/2 -translate-y-1/2 text-muted min-[700px]:hidden" onClick={() => { setMobileOpen(false); setShow(false) }}><X size={16} /></button>}
        {show && term.length > 0 && (
          <div className="absolute right-0 top-full z-50 mt-1 w-full min-w-72 overflow-hidden rounded-lg border border-line bg-panel shadow-xl max-[699px]:min-w-0">
            {res.isError && <div className="px-3 py-2 text-sm text-loss">Search failed.</div>}
            {res.data && rows.length === 0 && <div className="px-3 py-2 text-sm text-muted">No players found.</div>}
            {rows.map((p) => (
              <button key={p.id} type="button" className="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-panel2" onClick={() => pick(p.id)}>
                <Avatar id={p.id} name={p.name} photo={p.photo} color={p.team?.primary} size={24} />
                <span className="min-w-0 flex-1 truncate font-medium">{p.name}</span>
                <span className="text-xs text-muted">{p.team?.abbr ?? 'FA'}</span>
                <span className="w-7 text-right font-bold tabular-nums" style={{ color: ovrColor(p.ovr) }}>{p.ovr}</span>
              </button>
            ))}
            {res.data && res.data.total > 0 && (
              <button type="button" className="block w-full border-t border-line px-3 py-1.5 text-left text-xs font-semibold text-accent hover:bg-panel2" onClick={seeAll}>
                See all {res.data.total} {res.data.total === 1 ? 'result' : 'results'}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
