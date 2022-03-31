import * as React from 'react'
import {useState} from 'react'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import {ConfigType, FeaturesQuery, useConfigurationQuery} from '../../lib/schema/graphql'
import {Table} from '@navikt/ds-react'
import {Add, Delete, FileContent, Globe, Warning, Wrench} from '@navikt/ds-icons'
import styled from 'styled-components'
import {navGronn, navRod} from '../../styles/constants'
import ConfigAdd from '../lib/configAdd'
import ConfigEdit from '../lib/configEdit'
import ConfigDelete from '../lib/configDelete'
import prettifyArray from '../lib/prettifyArray'
import ConfigRows from "../lib/configRows";

const Center = styled.div`
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
    required: boolean
}

interface Configs {
    [index: string]: Config
}

interface ConfigProps {
    feature: FeaturesQuery['features'][0],
}

const ConfigPage = ({feature}: ConfigProps) => {
    const {data, error, loading} = useConfigurationQuery({variables: {feature: feature.name}})
    const [currentConfig, setCurrentConfig] = useState<Config | undefined>()
    const [showDelete, setShowDelete] = useState(false)
    const [showUpdate, setShowUpdate] = useState(false)
    const [showCreate, setShowCreate] = useState(false)

    let configs: Configs = {}
    if (data) {
        data.configuration.forEach((c) => {
            configs[c.key] = {...c, secret: false, env: false, type: ConfigType.Bool, required: false}
        })
        Object.keys(feature.config).forEach((k) => {
                if (!configs[k]) {
                    configs[k] = {
                        value: null,
                        env: false,
                        feature: feature.name,
                        key: k,
                        type: feature.config[k].type,
                        secret: feature.config[k].secret,
                        required: feature.config[k].required
                    }
                } else {
                    configs[k].type = feature.config[k].type
                    configs[k].secret = feature.config[k].secret
                    configs[k].required = feature.config[k].required
                }

            }
        )

    }

    const resetConfig = () => {
        setCurrentConfig(undefined)
        setShowDelete(false)
        setShowCreate(false)
        setShowUpdate(false)
    }

    const keys = Object.keys(configs).sort()

    return (
        <>
            {loading && <LoaderSpinner/>}
            {error && <ErrorMessage error={error}/>}
            <Table size={'small'}>
                <Table.Header>
                    <Table.Row>
                        <Table.HeaderCell>Key</Table.HeaderCell>
                        <Table.HeaderCell>Value</Table.HeaderCell>
                        <Table.HeaderCell>Scope</Table.HeaderCell>
                        <Table.HeaderCell>Required</Table.HeaderCell>
                        <Table.HeaderCell align='center'>Operations</Table.HeaderCell>
                        <Table.HeaderCell>Comment</Table.HeaderCell>
                    </Table.Row>
                </Table.Header>
                <Table.Body>
                    <ConfigRows
                        configs={configs}
                        keys={keys}
                        setCurrentConfig={setCurrentConfig}
                        setShowUpdate={setShowUpdate}
                        setShowDelete={setShowDelete}
                        setShowCreate={setShowCreate}
                        featurePage={true}
                    />
                </Table.Body>
            </Table>

            {currentConfig &&
                <>
                    <ConfigAdd conf={currentConfig} open={showCreate} showOpen={resetConfig}/>
                    <ConfigEdit conf={currentConfig} open={showUpdate} showOpen={resetConfig}/>
                    <ConfigDelete conf={currentConfig} open={showDelete} resetState={resetConfig}/>
                </>
            }
        </>
    )
}
export default ConfigPage
