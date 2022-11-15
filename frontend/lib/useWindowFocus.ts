import { useEffect, useState } from 'react'

export namespace useWindowFocus {
  export interface Return {
    /** Whether the user's cursor is currently focused in this window. */
    isWindowFocused: boolean
  }
}

/**
 * Hook that returns whether the window is currently focused. Re-evaluates whenever the window's "is
 * focused" state changes.
 */
// Note: Inspired by https://github.com/jpalumickas/use-window-focus/blob/main/src/index.ts.
export function useWindowFocus(): useWindowFocus.Return {
  const [isWindowFocused, setIsWindowFocused] = useState(hasFocus()) // Focus for first render.

  useEffect(() => {
    setIsWindowFocused(hasFocus()) // Focus for following renders.

    const onFocus = () => setIsWindowFocused(true)
    const onBlur = () => setIsWindowFocused(false)

    window.addEventListener('focus', onFocus)
    window.addEventListener('blur', onBlur)

    return () => {
      window.removeEventListener('focus', onFocus)
      window.removeEventListener('blur', onBlur)
    }
  }, [])

  return { isWindowFocused }
}

function hasFocus() {
  return document.hasFocus()
}
