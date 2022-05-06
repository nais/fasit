import * as React from 'react'
import {useState} from 'react'
import styled from 'styled-components'
import ConfigPage from './configPage'
import {
    EnvironmentGetQuery,
    FeaturesQuery,
    RolloutStatus, useConfigGetQuery,
    useFeaturesQuery,
    useFeatureStatusQuery
} from '../../lib/schema/graphql'
import {navGronn, navOransje, navRod} from '../../styles/constants'
import IconBox from "../lib/icons/iconBox";
import GitIcon from "../lib/icons/gitIcon";
import {Loader, Switch} from '@navikt/ds-react'
import EnableFeature from "./enableFeature";
import {Configs} from "../lib/configRows";


const FeatureContainer = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
`
const EmptyFeature = styled.div`
  border-left: 1px solid silver;
`

const FeatureStatus = styled.div`
  border: 1px solid silver;
  border-radius: 5px;
  padding: 10px;
  background-color: #f5f5f5;
  font-size: 0.8em;
  margin-bottom: 10px;
`

const EnableFeatureBox = styled.div`
  display: flex;
  align-items: center;
  justify-self: center;
  flex-direction: column;
`

interface StatusIndicatorProps {
    status: RolloutStatus | undefined
}

const StatusIndicator = styled.div<StatusIndicatorProps>`
width: 10px;
height: 10px;
border-radius: 50%;
margin: 0 5px;
${(props) => {
    switch (props.status) {
        case RolloutStatus.Deployed:
            return `background-color: ${navGronn};`
        case RolloutStatus.Failed:
            return `background-color: ${navRod};`
        case RolloutStatus.Pending:
            return `background-color: ${navOransje};`
        case RolloutStatus.Unknown:
        default:
            return `background-color: #222;`
    }
}} `
const StatusField = styled.div`
display: flex;
align-items: center;

`

interface FeatureProps {
    env: EnvironmentGetQuery['environment']
    featureName: string,
}

const Feature = ({env, featureName}: FeatureProps) => {
    const {loading, error, data} = useFeatureStatusQuery({variables: {envID: env.id, feature: featureName}})
    const configQuery = useConfigGetQuery({variables: {envID: env.id, feature: featureName}})
    const features = useFeaturesQuery({variables: {kind: env.kind}})

    let configs: Configs = {}
    let featureObject: FeaturesQuery['features'][0] | undefined
    if (features.data && configQuery.data) {
        featureObject = features.data.features.find((f) => f.name === featureName)
        const confKeys = featureObject?.config
        configQuery.data?.envConfig.forEach((c) => {
            if (c.__typename === 'EnvConfiguration') {
                configs[c.key] = {
                    id: c.id,
                    feature: c.feature.name,
                    key: c.key,
                    type: c.type,
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
                    type: c.type,
                    value: c.value,
                    description: c.description,
                    secret: false,
                    required: false,
                    enabled: false,
                    env: false,
                }
            }
        })
        Object.keys(confKeys).forEach((k) => {
                if (!configs[k]) {
                    configs[k] = {
                        value: null,
                        env: false,
                        feature: featureName,
                        key: k,
                        type: confKeys[k].type,
                        secret: confKeys[k].secret,
                        required: confKeys[k].required,
                        enabled: confKeys[k].enabled
                    }
                } else {
                    configs[k].type = confKeys[k].type
                    configs[k].secret = confKeys[k].secret
                    configs[k].required = confKeys[k].required
                    configs[k].enabled = confKeys[k].enabled
                }
            },
        )
    }


    const requiredConfigs = Object.keys(configs).filter((c) => configs[c].required).sort()
    const featureState = env.featureStates.find((f) => f.feature.name === featureName)
    if (!env || !featureState) {
        return <EmptyFeature/>
    }

    const f = featureState.feature
    const [showVerify, setShowVerify] = useState(false)
    const missingRequirements = requiredConfigs.filter((r) => !configs[r].value)
    const missingDependencies = f.dependsOn.filter((dependency) => {
        return !env.featureStates.find((fs) => fs.feature.name === dependency)?.enabled
    })
    const status = data?.featureStatus

    return (
        <FeatureContainer>
            <FeatureStatus>
                <div style={{display: 'flex', alignItems: 'center'}} key={f.name}>
                    <div key={f.name} style={{display: 'flex', flexDirection: 'column', flexGrow: '1'}}>
                        <div>
                            {loading && <StatusField><Loader transparent/></StatusField>}
                            {error && <StatusField>status: <StatusIndicator
                                status={status?.status}/>{error.message === "sql: no rows in result set" ? "No status" : error.message}
                            </StatusField>}
                            {data && <>
                                <StatusField>status: <StatusIndicator
                                    status={status?.status}/>{status?.status.toLowerCase()}</StatusField>
                            </>
                            }
                        </div>
                        {f.chart && <div>chart: {f.chart}</div>}
                        {f.repo && <div>repo: {f.repo}</div>}
                        {f.version && <div>version: {f.version}</div>}
                        {f.source && <div style={{display: 'flex', width: 'fit-content', gap: '10px'}}><IconBox
                            size={20}><GitIcon/></IconBox> <a href={f.source} target="_blank">{f.source}</a></div>}
                        {f.dependsOn.length > 0 && <div>dependencies: {f.dependsOn.map((d) => {
                            return <span key={d}
                                         style={{color: missingDependencies.includes(d) ? navRod : navGronn}}>{d + " "}</span>
                        })}
                        </div>
                        }
                    </div>
                    <EnableFeatureBox>
                        <div>Enabled</div>
                        <Switch disabled={missingDependencies.length > 0 || missingRequirements.length > 0} size="medium"
                                checked={featureState.enabled}
                                onChange={() => setShowVerify(true)}>{''}</Switch>
                        {missingDependencies.length > 0 && "Missing dependencies"}
                    </EnableFeatureBox>
                </div>
            </FeatureStatus>
            {
                featureName && env && <ConfigPage envID={env.id} configs={configs} featureObject={featureObject}/>
            }
            <EnableFeature open={showVerify} onClose={setShowVerify} feature={f.name} envID={env.id}
                           enabled={featureState.enabled}/>
        </FeatureContainer>
    )
}
export default Feature