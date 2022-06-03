import * as React from 'react'
import {useState} from 'react'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import {ConfigType, FeaturesQuery, useConfigurationQuery} from '../../lib/schema/graphql'
import {Table} from '@navikt/ds-react'
import ConfigAdd from '../lib/configAdd'
import ConfigEdit from '../lib/configEdit'
import ConfigDelete from '../lib/configDelete'
import ConfigRows, {Config, Configs} from "../lib/configRows";


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
        data.configuration.configuration.forEach((c) => {
            if (c.__typename === 'EnvConfiguration') {
                configs[c.key] = {
                id: c.id,
                feature: c.feature.name,
                key: c.key,
                type: ConfigType.Bool,
                value: c.value,
                description: c.description,
                secret: false,
                required: false,
                enabled: false,
                env: true,
                }
            } else {
                configs[c.key] = {
                id: c.id,
                feature: c.feature.name,
                key: c.key,
                type: ConfigType.Bool,
                value: c.value,
                description: c.description,
                secret: false,
                required: false,
                enabled: false,
                env: false,
                }
            }
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
                    <ConfigAdd conf={currentConfig} feature={feature} open={showCreate} showOpen={resetConfig}/>
                    <ConfigEdit conf={currentConfig} open={showUpdate} showOpen={resetConfig}/>
                    <ConfigDelete conf={currentConfig} open={showDelete} resetState={resetConfig}/>
                </>
            }
        </>
    )
}
export default ConfigPage
