import { Table } from '@navikt/ds-react'
import {
  ConfigType,
  Configuration,
  EnvConfiguration,
  GlobalConfiguration,
} from '../../lib/schema/graphql'
import prettifyArray from './prettifyArray'
import {
  Add,
  Delete,
  FileContent,
  Globe,
  Place,
  Success,
  Warning,
  Wrench,
} from '@navikt/ds-icons'
import { navGronn, navRod } from '../../styles/constants'
import * as React from 'react'
import styled from 'styled-components'
import ReactTooltip from 'react-tooltip'
import ValuesCollapse from '../tenant/valuesCollapse'

const Center = styled.div`
  display: flex;
  gap: 10px;
  justify-content: center;
`

const StyledWrench = styled(Wrench)`
  :hover {
    color: ${navGronn};
  }
  cursor: pointer;
`

const StyledDelete = styled(Delete)`
  :hover {
    color: ${navRod};
  }
  cursor: pointer;
`

const StyledAdd = styled(Add)`
  :hover {
    color: ${navGronn};
  }
  cursor: pointer;
`
const StyledValue = styled.pre`
  font-size: 0.6rem;
  overflow: auto;
  width: inherit;
`

const StyledConfigKey = styled.span``

export type Config = {
  __typename?: 'EnvConfiguration' | 'GlobalConfiguration'
  id: string
  description: string
  displayName: string
  value: any
  type: ConfigType
  key: string
  secret: boolean
  required: boolean
  enabled?: boolean
  chartValue: any
}

export interface Configs {
  [index: string]: Config
}

interface ConfigRowProps {
  configs: Array<Config>
  setCurrentConfig: React.Dispatch<Config>
  setShowUpdate: React.Dispatch<boolean>
  setShowDelete: React.Dispatch<boolean>
  setShowCreate: React.Dispatch<boolean>
  featurePage?: boolean
}

const ConfigRows = ({
  configs,
  setCurrentConfig,
  setShowUpdate,
  setShowCreate,
  setShowDelete,
  featurePage,
}: ConfigRowProps) => {
  const display = (conf: Config) => {
    if (conf.secret) {
      return '******'
    } else if (conf.type === ConfigType.StringArray) {
      return prettifyArray(conf.value)
    } else if (conf.value != null) {
      if (conf.value.toString().trim().startsWith('{')) {
        const obj = JSON.parse(conf.value)
        return <StyledValue>{JSON.stringify(obj, null, 2)}</StyledValue>
      }
      return JSON.stringify(conf.value).replace(/"/g, '')
    }
    return (
      <ValuesCollapse
        content={{
          value: conf.chartValue || 'empty?',
          displayName: 'asd',
          key: conf.key,
        }}
      />
    )
  }
  return (
    <>
      {configs.map((conf, i) => {
        return (
          <Table.Row key={i}>
            <Table.DataCell align={'center'}>
              {' '}
              {conf.__typename === 'EnvConfiguration' ? (
                <Place data-tip data-for="local" />
              ) : JSON.stringify(conf.value) !== 'null' ? (
                <Globe data-tip data-for="global" />
              ) : (
                <FileContent data-tip data-for="helm" />
              )}
              <ReactTooltip id="local" place="top" type="dark" effect="solid">
                {'Local value'}
              </ReactTooltip>
              <ReactTooltip id="global" place="top" type="dark" effect="solid">
                {'Global value'}
              </ReactTooltip>
              <ReactTooltip id="helm" place="top" type="dark" effect="solid">
                {'Helm default'}
              </ReactTooltip>
            </Table.DataCell>
            <Table.DataCell align={'center'}>
              {conf.required && (
                <Center>
                  {conf.value ? (
                    <Success
                      style={{ color: navGronn }}
                      title={'Requirement met'}
                    />
                  ) : (
                    <Warning
                      style={{ color: navRod }}
                      title={'required field'}
                    />
                  )}
                </Center>
              )}
            </Table.DataCell>
            <Table.DataCell>
              <StyledConfigKey data-tip data-for={conf.key}>
                {conf.displayName ? (
                  <span title={'helm key: ' + conf.key}>
                    {conf.displayName}
                  </span>
                ) : (
                  conf.key
                )}
              </StyledConfigKey>
              {conf.description ? (
                <ReactTooltip id={conf.key} aria-haspopup="true">
                  <p>{conf.description}</p>
                </ReactTooltip>
              ) : (
                ''
              )}
            </Table.DataCell>
            <Table.DataCell style={{ maxWidth: '40vw' }}>
              {display(conf)}
            </Table.DataCell>
            <Table.DataCell align={'center'}>
              <Center>
                {' '}
                {conf.__typename === 'EnvConfiguration' ||
                (featurePage && conf.value != null) ? (
                  <>
                    <StyledWrench
                      data-tip
                      data-for="modify"
                      onClick={() => {
                        setCurrentConfig(conf)
                        setShowUpdate(true)
                      }}
                    />
                    <StyledDelete
                      data-tip
                      data-for="delete"
                      onClick={() => {
                        setCurrentConfig(conf)
                        setShowDelete(true)
                      }}
                    />
                    <ReactTooltip
                      id="modify"
                      place="top"
                      type="dark"
                      effect="solid"
                    >
                      {'Modify'}
                    </ReactTooltip>
                    <ReactTooltip
                      id="delete"
                      place="top"
                      type="dark"
                      effect="solid"
                    >
                      {'Delete'}
                    </ReactTooltip>
                  </>
                ) : (
                  <>
                    <StyledAdd
                      data-tip
                      data-for="add"
                      onClick={() => {
                        setCurrentConfig(conf)
                        setShowCreate(true)
                      }}
                    />
                    <ReactTooltip
                      id="add"
                      place="top"
                      type="dark"
                      effect="solid"
                    >
                      {'Add'}
                    </ReactTooltip>
                  </>
                )}
              </Center>
            </Table.DataCell>
          </Table.Row>
        )
      })}
    </>
  )
}
export default ConfigRows
