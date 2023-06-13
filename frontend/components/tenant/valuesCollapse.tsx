import React from 'react'
import useCollapse from 'react-collapsed'
import styled from 'styled-components'
import { Expand, Collapse } from '@navikt/ds-icons'
import { MappingValue } from '../../lib/schema/graphql'
import { Tooltip } from 'react-tooltip'

const LogPre = styled.pre`
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 12px;
`

const Header = styled.div`
  cursor: pointer;
  text-align: left;
`

interface DropdownProps {
  content: MappingValue
}

const isJson = (x: any) => {
  try {
    JSON.parse(x)
  } catch (e) {
    return false
  }
  return true
}

const isObject = (x: any) => {
  return x.toString() === '[object Object]'
}

function safeToString(x: any) {
  if (typeof x === 'string') {
    return x
  }

  if (isJson(x) || isObject(x)) {
    return JSON.stringify(x, null, 2)
  }

  return x.toString()
}

const ValuesCollapse = ({ content }: DropdownProps) => {
  const { getCollapseProps, getToggleProps, isExpanded } = useCollapse()
  return Array.isArray(content.value) && content.value.length > 1 ? (
    <div className="collapsible">
      <Header {...getToggleProps()}>
        {isExpanded ? (
          <>
            <Collapse data-tip data-for={'collapse'} />
            <Tooltip id="collapse" place="top" variant="dark">
              {' '}
              Collapse{' '}
            </Tooltip>
          </>
        ) : (
          <>
            <Expand data-tip data-for={'expand'} />
            <Tooltip id="expand" place="top" variant="dark">
              {' '}
              Expand{' '}
            </Tooltip>
          </>
        )}
      </Header>
      <div {...getCollapseProps()}>
        <LogPre>{JSON.stringify(content.value, null, 2)}</LogPre>
      </div>
    </div>
  ) : (
    safeToString(content.value)
  )
}
export default ValuesCollapse
