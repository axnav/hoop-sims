import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { Card, Page, Q } from '../ui'
import { NewsStory, newsDate } from '../NewsCard'

export default function News() {
  const [filter, setFilter] = useState('all')
  const [limit, setLimit] = useState(100)
  const q = useQuery({ queryKey: ['news', filter, limit], queryFn: () => api.news(filter, limit), placeholderData: (prev) => prev })
  return (
    <Page title="League news">
      <Q q={q} minH={300}>
        {(d) => {
          const items = d.items ?? []
          const filters = d.filters ?? []
          let last = ''
          return (
            <>
              {filters.length > 0 && (
                <div className="mb-3 flex flex-wrap gap-1.5">
                  {filters.map((f) => (
                    <button key={f.key} className={`btn ${f.key === filter ? 'btn-accent' : ''}`} aria-pressed={f.key === filter} onClick={() => { setFilter(f.key); setLimit(100) }}>
                      {f.label} <span className="tabular-nums opacity-70">{f.count}</span>
                    </button>
                  ))}
                </div>
              )}
              {items.length === 0 ? (
                <div className="card p-8 text-center text-muted">No news yet. Play some games.</div>
              ) : (
                <Card>
                  <div>
                    {items.map((n) => {
                      const date = newsDate(n)
                      const head = date !== last
                      last = date
                      return (
                        <div key={n.id}>
                          {head && <div className="h2 border-b border-line bg-panel2/40 px-3 py-1.5">{date}</div>}
                          <NewsStory n={n} />
                        </div>
                      )
                    })}
                  </div>
                </Card>
              )}
              {d.total > items.length && limit < 400 && (
                <div className="mt-3 text-center"><button className="btn" onClick={() => setLimit((l) => Math.min(400, l + 100))}>Show more</button></div>
              )}
            </>
          )
        }}
      </Q>
    </Page>
  )
}
