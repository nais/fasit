import { useEffect } from 'react'

import { useWindowFocus } from './useWindowFocus'

export namespace useFocusPoll {
  export interface Args {
    pollInterval: number

    stopped?: boolean

    /** The `refetch` function returned from `useQuery`. */
    refetch: () => void

    /** The `startPolling` function returned from `useQuery`. */
    startPolling: (pollInterval: number) => void

    /** The `stopPolling` function returned from `useQuery`. */
    stopPolling: () => void
  }
}

/**
 * Hook that enables polling for a given GraphQL query while the window is focused - and disables
 * polling while the window is not focused. This reduces network traffic to our server while the
 * user isn't literally focused on our application.
 *
 * See the [Apollo docs](https://www.apollographql.com/docs/react/data/queries/#polling) for details
 * about polling.
 */
export function useFocusPoll({
  pollInterval,
  stopped,
  refetch,
  startPolling,
  stopPolling,
}: useFocusPoll.Args): void {
  const { isWindowFocused } = useWindowFocus()

  useEffect(() => {
    if (!isWindowFocused || stopped) {
      stopPolling()
    } else {
      // Refetch data immediately when the window is refocused.
      refetch?.()
      startPolling(pollInterval)
    }
  }, [
    isWindowFocused,
    pollInterval,
    stopped,
    refetch,
    startPolling,
    stopPolling,
  ])
}
