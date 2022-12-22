import * as React from 'react'
import styled from 'styled-components'
import humanizeDate from '../lib/humanizeDate'

const LogPre = styled.pre`
  overflow: auto;
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 14px;
`

const LogLine = styled.div`
  position: relative;
  margin-bottom: 5px;

  & > time {
    color: #999;
    background-color: white;
    font-size: 0.9em;
    position: absolute;
    top: 0.1em;
    right: 0.1em;
  }

  &:hover > time {
    color: #666;
  }
`

interface FeatureLogsViewProps {
  logs: string
}

const FeatureLogsView = ({ logs }: FeatureLogsViewProps) => {
  if (logs != '' && logs[0] == '[') {
    const lines = JSON.parse(logs)
    return (
      <LogPre>
        {lines.map((line: { msg: string; time: string }, i: number) => (
          <LogLine key={i}>
            {humanizeDate(line.time, 'dd-MM-yyyy HH:mm:ss')}
            {line.msg}
          </LogLine>
        ))}
      </LogPre>
    )
  }

  return (
    <>
      <LogPre>{logs ? logs : 'No logs available'}</LogPre>
    </>
  )
}
export default FeatureLogsView
