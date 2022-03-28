import * as React from 'react'
import { useState } from 'react'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import { ConfigType, useConfigGetQuery, useFeaturesQuery } from '../../lib/schema/graphql'
import { Table } from '@navikt/ds-react'
import { Add, Delete, FileContent, Globe, Place, Wrench } from '@navikt/ds-icons'
import styled from 'styled-components'
import { navGronn, navRod } from '../../styles/constants'
import ConfigAdd from '../lib/configAdd'
import ConfigDelete from '../lib/configDelete'
import ConfigEdit from '../lib/configEdit'
import prettifyArray from '../lib/prettifyArray'


const Operations = styled.div`
  display: flex;
  gap: 10px;
  justify-content: center;
`

const StyledWrench = styled(Wrench)`
  :hover{
    color: ${navGronn};
  }
  cursor: pointer;
`

const StyledDelete = styled(Delete)`
  :hover{
    color: ${navRod};
  }
  cursor: pointer;
`

const StyledAdd = styled(Add)`
  :hover{
    color: ${navGronn};
  }
  cursor: pointer;
`

export interface Config {
  id?: string
  description?: string | null
  value: any
  type: ConfigType
  env: boolean
  feature: string
  key: string
  secret: boolean
}

interface Configs {
  [index: string]: Config
}

interface ConfigProps {
  envID: string,
  feature: string,
}

const ConfigPage = ({ envID, feature }: ConfigProps) => {
  const { data, error, loading } = useConfigGetQuery({ variables: { envID, feature } })
  const features = useFeaturesQuery()
  const [currentConfig, setCurrentConfig] = useState<Config | undefined>()
  const [showDelete, setShowDelete] = useState(false)
  const [showUpdate, setShowUpdate] = useState(false)
  const [showCreate, setShowCreate] = useState(false)


  let configs: Configs = {}
  if (features.data && data) {
    const confKeys = features.data.features.find((f) => f.name === feature)?.config
    data.envConfig.forEach((c) => {
        configs[c.key] = { ...c, secret: false }
      },
    )
    Object.keys(confKeys).forEach((k) => {
        if (!configs[k]) {
          configs[k] = {
            value: null,
            env: false,
            feature: feature,
            key: k,
            type: confKeys[k].type,
            secret: confKeys[k].secret,
          }
        } else {
          configs[k].type = confKeys[k].type
          configs[k].secret = confKeys[k].secret
        }
      },
    )
  }

  const resetConfig = () => {
    setCurrentConfig(undefined)
    setShowDelete(false)
    setShowCreate(false)
    setShowUpdate(false)
  }

  return (
    <>
      {loading && <LoaderSpinner />}
      {error && <ErrorMessage error={error} />}
      <Table size={'small'}>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>Key</Table.HeaderCell>
            <Table.HeaderCell>Value</Table.HeaderCell>
            <Table.HeaderCell>Scope</Table.HeaderCell>
            <Table.HeaderCell>Comment</Table.HeaderCell>
            <Table.HeaderCell align='center'>Operations</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {
            Object.keys(configs).map((c) => {
                const conf = configs[c]
                return (
                  <Table.Row key={c}>
                    <Table.DataCell>{c}</Table.DataCell>
                    <Table.DataCell>{conf.type != ConfigType.StringArray ?
                      conf.value != null ? JSON.stringify(conf.value).replace(/"/g, '') :
                        '<default>' :
                      prettifyArray(conf.value)}
                    </Table.DataCell>
                    <Table.DataCell>{conf.env ? <Place /> : conf.value ? <Globe /> : <FileContent />}</Table.DataCell>
                    <Table.DataCell>{conf.description}</Table.DataCell>
                    <Table.DataCell>
                      <Operations> {conf.env ?
                        <>
                          <StyledWrench onClick={() => {
                            setCurrentConfig(conf)
                            setShowUpdate(true)
                          }}
                          />
                          <StyledDelete onClick={() => {
                            setCurrentConfig(conf)
                            setShowDelete(true)
                          }}
                          />
                        </> :
                        <StyledAdd onClick={() => {
                          setCurrentConfig(conf)
                          setShowCreate(true)
                        }} />}
                      </Operations>
                    </Table.DataCell>
                  </Table.Row>)
              },
            )
          }
        </Table.Body>
      </Table>

      {currentConfig &&
        <>
          <ConfigAdd conf={currentConfig} envID={envID} globalConfig={configs[currentConfig.key]} open={showCreate}
                     showOpen={resetConfig} />
          <ConfigEdit conf={currentConfig} open={showUpdate} showOpen={resetConfig} />
          <ConfigDelete conf={currentConfig} open={showDelete} resetState={resetConfig} />
        </>
      }
    </>
  )
}
export default ConfigPage
