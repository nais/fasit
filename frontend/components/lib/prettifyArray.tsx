import * as React from 'react'

const prettifyArray = (arr: []) => {
  return <>
    {
      !arr ? '<default>'  :
        arr.map((e) => {
          return <div key={e} style={{ fontFamily: 'monospace', fontSize: '0.8em' }}>{e}</div>
        })
    }
  </>
}
export default prettifyArray
