import { AutomaticSystem, ExpandFilled, Wrench } from '@navikt/ds-icons'
import { Table } from '@navikt/ds-react'
import { useEffect, useState } from 'react'
import ReactTooltip from 'react-tooltip'
import styled from 'styled-components'
import { ConfigurationQuery, FeaturesQuery } from '../../lib/schema/graphql'
import { navBla, navGronn } from '../../styles/constants'
import ConfigAdd from '../lib/configAdd'
import ConfigDelete from '../lib/configDelete'
import ConfigEdit from '../lib/configEdit'
import ConfigRows, { Config, Configs } from '../lib/configRows'
import ValuesCollapse from './valuesCollapse'

interface FeatureConfigProps {
  envID: string
  configs: Configs
  featureObject: FeaturesQuery['features'][0] | undefined
  mapping?: ConfigurationQuery['configuration']['mapping']
}

const StyledWrench = styled(Wrench)`
  :hover {
    color: ${navGronn};
  }
  cursor: pointer;
`

const FlatButton = styled.button`
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  cursor: pointer;
  outline: inherit;
  width: 100%;

  &:hover {
    color: ${navBla};
  }
`

const FeatureConfig = ({
  envID,
  configs,
  featureObject,
  mapping,
}: FeatureConfigProps) => {
  const [currentConfig, setCurrentConfig] = useState<Config | undefined>()
  const [showDelete, setShowDelete] = useState(false)
  const [showUpdate, setShowUpdate] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [showMapping, setShowMapping] = useState(false)

  useEffect(() => {
    if (showMapping) {
      return function cleanup() {
        setShowMapping(false)
      }
    }
  })

  const overridable = Object.keys(configs).filter((c) =>
    mapping?.map((m) => m.key).includes(configs[c].key),
  )
  const nonOverridden = Object.keys(configs)
    .filter((c) => overridable?.includes(c))
    .filter((c) => !configs[c].value)

  const requiredConfigs = Object.keys(configs)
    .filter((c) => configs[c].required)
    .filter((c) => !nonOverridden.includes(c))
    .sort()
  const envConfigs = Object.keys(configs)
    .filter((c) => configs[c].env && !configs[c].required)
    .sort()
  const theRest = Object.keys(configs)
    .filter((c) => !configs[c].env && !configs[c].required)
    .sort()

  const resetConfig = () => {
    setCurrentConfig(undefined)
    setShowDelete(false)
    setShowCreate(false)
    setShowUpdate(false)
  }

  const filteredMappings = mapping?.filter((m) => {
    return !(configs[m.key] && configs[m.key].value !== null)
  })

  return (
    <>
      <Table size={'small'}>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              align={'center'}
              style={{ width: '50px' }}
            ></Table.HeaderCell>
            <Table.HeaderCell
              align={'center'}
              style={{ width: '50px' }}
            ></Table.HeaderCell>
            <Table.HeaderCell style={{ width: '200px' }}>Key</Table.HeaderCell>
            <Table.HeaderCell>Value</Table.HeaderCell>
            <Table.HeaderCell style={{ width: '100px' }} align="center">
              Actions
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {requiredConfigs.length > 0 && (
            <ConfigRows
              configs={configs}
              keys={requiredConfigs}
              setCurrentConfig={setCurrentConfig}
              setShowUpdate={setShowUpdate}
              setShowDelete={setShowDelete}
              setShowCreate={setShowCreate}
            />
          )}
          {envConfigs.length > 0 && (
            <ConfigRows
              configs={configs}
              keys={envConfigs}
              setCurrentConfig={setCurrentConfig}
              setShowUpdate={setShowUpdate}
              setShowDelete={setShowDelete}
              setShowCreate={setShowCreate}
            />
          )}
          {theRest.length > 0 && (
            <ConfigRows
              configs={configs}
              keys={theRest}
              setCurrentConfig={setCurrentConfig}
              setShowUpdate={setShowUpdate}
              setShowDelete={setShowDelete}
              setShowCreate={setShowCreate}
            />
          )}
          {filteredMappings && filteredMappings.length > 0 && !showMapping && (
            <Table.Row>
              <Table.DataCell colSpan={5} align="center">
                <FlatButton onClick={() => setShowMapping(true)}>
                  <ExpandFilled height="20px" width="20px" />
                  Show {filteredMappings.length} mapping value
                  {filteredMappings.length > 1 ? 's' : ''}
                </FlatButton>
              </Table.DataCell>
            </Table.Row>
          )}
          {filteredMappings &&
            showMapping &&
            filteredMappings.map((m) => {
              const o = overridable.includes(m.key)
              return (
                <Table.Row key={m.key}>
                  <Table.DataCell align={'center'}>
                    <AutomaticSystem
                      data-tip
                      data-for="mapping"
                      title={'Mapping value'}
                    />
                    <ReactTooltip
                      id="mapping"
                      place="top"
                      type="dark"
                      effect="solid"
                    >
                      {'Mapping value'}
                    </ReactTooltip>
                  </Table.DataCell>
                  <Table.DataCell />
                  <Table.DataCell style={{ overflowWrap: 'break-word' }}>
                    {m.displayName ? (
                      <span title={'helm key: ' + m.key}>{m.displayName}</span>
                    ) : (
                      m.key
                    )}
                  </Table.DataCell>
                  <Table.DataCell>
                    <ValuesCollapse content={m} />
                  </Table.DataCell>
                  <Table.DataCell align={'center'}>
                    {' '}
                    {o && (
                      <StyledWrench
                        onClick={() => {
                          setCurrentConfig(configs[m.key])
                          setShowCreate(true)
                        }}
                      />
                    )}
                  </Table.DataCell>
                </Table.Row>
              )
            })}
        </Table.Body>
      </Table>

      {currentConfig && featureObject && (
        <>
          <ConfigAdd
            conf={currentConfig}
            envID={envID}
            globalConfig={configs[currentConfig.key]}
            open={showCreate}
            feature={featureObject}
            showOpen={resetConfig}
          />
          <ConfigEdit
            conf={currentConfig}
            open={showUpdate}
            showOpen={resetConfig}
          />
          <ConfigDelete
            conf={currentConfig}
            open={showDelete}
            resetState={resetConfig}
          />
        </>
      )}
    </>
  )
}
export default FeatureConfig
