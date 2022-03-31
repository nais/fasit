import * as React from 'react'
import {useState} from 'react'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import {ConfigType, useConfigGetQuery, useEnvironmentGetQuery, useFeaturesQuery} from '../../lib/schema/graphql'
import {Table} from '@navikt/ds-react'
import ConfigAdd from '../lib/configAdd'
import ConfigDelete from '../lib/configDelete'
import ConfigEdit from '../lib/configEdit'
import ConfigRows from "../lib/configRows";

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

export interface Configs {
    [index: string]: Config
}

interface ConfigProps {
    envID: string,
    feature: string,
}

const ConfigPage = ({envID, feature}: ConfigProps) => {
    const {data, error, loading} = useConfigGetQuery({variables: {envID, feature}})
    const features = useFeaturesQuery()
    const [currentConfig, setCurrentConfig] = useState<Config | undefined>()
    const [showDelete, setShowDelete] = useState(false)
    const [showUpdate, setShowUpdate] = useState(false)
    const [showCreate, setShowCreate] = useState(false)


    let configs: Configs = {}
    if (features.data && data) {
        const confKeys = features.data.features.find((f) => f.name === feature)?.config
        data.envConfig.forEach((c) => {
                configs[c.key] = {...c, secret: false, required: false}
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
                        required: confKeys[k].required
                    }
                } else {
                    configs[k].type = confKeys[k].type
                    configs[k].secret = confKeys[k].secret
                    configs[k].required = confKeys[k].required
                }
            },
        )
    }
    const requiredConfigs = Object.keys(configs).filter((c) => configs[c].required).sort()
    const envConfigs = Object.keys(configs).filter((c) => configs[c].env && !configs[c].required).sort()
    const theRest = Object.keys(configs).filter((c) => !configs[c].env && !configs[c].required).sort()


    const resetConfig = () => {
        setCurrentConfig(undefined)
        setShowDelete(false)
        setShowCreate(false)
        setShowUpdate(false)
    }

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
            {requiredConfigs.length > 0 &&
                    <ConfigRows
                        configs={configs}
                        keys={requiredConfigs}
                        setCurrentConfig={setCurrentConfig}
                        setShowUpdate={setShowUpdate}
                        setShowDelete={setShowDelete}
                        setShowCreate={setShowCreate}
                    />
            }
            {envConfigs.length > 0 &&
                    <ConfigRows
                        configs={configs}
                        keys={envConfigs}
                        setCurrentConfig={setCurrentConfig}
                        setShowUpdate={setShowUpdate}
                        setShowDelete={setShowDelete}
                        setShowCreate={setShowCreate}
                    />
            }
            {theRest.length > 0 &&
                    <ConfigRows
                        configs={configs}
                        keys={theRest}
                        setCurrentConfig={setCurrentConfig}
                        setShowUpdate={setShowUpdate}
                        setShowDelete={setShowDelete}
                        setShowCreate={setShowCreate}
                    />
            }
                </Table.Body>
            </Table>

            {currentConfig &&
                <>
                    <ConfigAdd conf={currentConfig} envID={envID} globalConfig={configs[currentConfig.key]}
                               open={showCreate}
                               showOpen={resetConfig}/>
                    <ConfigEdit conf={currentConfig} open={showUpdate} showOpen={resetConfig}/>
                    <ConfigDelete conf={currentConfig} open={showDelete} resetState={resetConfig}/>
                </>
            }
        </>
    )
}
export default ConfigPage
