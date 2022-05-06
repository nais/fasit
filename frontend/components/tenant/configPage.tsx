import * as React from 'react'
import {useState} from 'react'
import {FeaturesQuery} from '../../lib/schema/graphql'
import {Table} from '@navikt/ds-react'
import ConfigAdd from '../lib/configAdd'
import ConfigDelete from '../lib/configDelete'
import ConfigEdit from '../lib/configEdit'
import ConfigRows, {Config, Configs} from "../lib/configRows";


interface ConfigProps {
    envID: string
    configs: Configs
    featureObject: FeaturesQuery['features'][0] | undefined
}

const ConfigPage = ({envID, configs, featureObject}: ConfigProps) => {
    const [currentConfig, setCurrentConfig] = useState<Config | undefined>()
    const [showDelete, setShowDelete] = useState(false)
    const [showUpdate, setShowUpdate] = useState(false)
    const [showCreate, setShowCreate] = useState(false)

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
            <Table size={'small'}>
                <Table.Header>
                    <Table.Row>
                        <Table.HeaderCell>Key</Table.HeaderCell>
                        <Table.HeaderCell>Value</Table.HeaderCell>
                        <Table.HeaderCell align={'center'}>Scope</Table.HeaderCell>
                        <Table.HeaderCell align={'center'}>Required</Table.HeaderCell>
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

            {currentConfig && featureObject &&
                <>
                    <ConfigAdd conf={currentConfig} envID={envID} globalConfig={configs[currentConfig.key]}
                               open={showCreate}
                               feature={featureObject}
                               showOpen={resetConfig}/>
                    <ConfigEdit conf={currentConfig} open={showUpdate} showOpen={resetConfig}/>
                    <ConfigDelete conf={currentConfig} open={showDelete} resetState={resetConfig}/>
                </>
            }
        </>
    )
}
export default ConfigPage
