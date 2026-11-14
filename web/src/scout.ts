import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from './api'

// Spend one scouting point on a prospect. Writes the new draft data into the cache and refreshes the player view.
export function useScout(onError?: (msg: string) => void) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (playerId: number) => api.draftScout(playerId),
    onSuccess: (d, playerId) => {
      onError?.('')
      qc.setQueryData(['draft'], d)
      qc.invalidateQueries({ queryKey: ['player', playerId] })
    },
    onError: (e) => onError?.(e instanceof Error ? e.message : String(e)),
  })
}
