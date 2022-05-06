import * as React from 'react'
import styled from 'styled-components'
import {EnvironmentGetQuery, RolloutStatus, useFeatureStatusQuery} from '../../lib/schema/graphql'
import {navGronn, navOransje, navRod} from '../../styles/constants'
import IconBox from "../lib/icons/iconBox";
import GitIcon from "../lib/icons/gitIcon";
import {Loader, Switch} from '@navikt/ds-react'
import {Configs} from "../lib/configRows";


const FeatureStatusContainer = styled.div`
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
    configs: Configs,
    env: EnvironmentGetQuery['environment']
    featureName: string
    setShowVerify: React.Dispatch<boolean>
}

const FeatureStatus = ({configs, env, featureName, setShowVerify}: FeatureProps) => {
    const {loading, error, data} = useFeatureStatusQuery({variables: {envID: env.id, feature: featureName}})

    const requiredConfigs = Object.keys(configs).filter((c) => configs[c].required).sort()
    const status = data?.featureStatus
    const featureState = env.featureStates.find((f) => f.feature.name === featureName)
    const feature = featureState!.feature

    const missingRequirements = requiredConfigs.filter((r) => !configs[r].value)
    const missingDependencies = feature.dependsOn.filter((dependency) => !env.featureStates.find((fs) => fs.feature.name === dependency)?.enabled )

    return (
        <FeatureStatusContainer>
            <div style={{display: 'flex', alignItems: 'center'}} key={feature.name}>
                <div key={feature.name} style={{display: 'flex', flexDirection: 'column', flexGrow: '1'}}>
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
                    {feature.chart && <div>chart: {feature.chart}</div>}
                    {feature.repo && <div>repo: {feature.repo}</div>}
                    {feature.version && <div>version: {feature.version}</div>}
                    {feature.source && <div style={{display: 'flex', width: 'fit-content', gap: '10px'}}><IconBox
                        size={20}><GitIcon/></IconBox> <a href={feature.source} target="_blank">{feature.source}</a></div>}
                    {feature.dependsOn.length > 0 && <div>dependencies: {feature.dependsOn.map((d) => {
                        return <span key={d}
                                     style={{color: missingDependencies.includes(d) ? navRod : navGronn}}>{d + " "}</span>
                    })}
                    </div>
                    }
                </div>
                <EnableFeatureBox>
                    <div>Enabled</div>
                    <Switch disabled={missingDependencies.length > 0 || missingRequirements.length > 0} size="medium"
                            checked={featureState?.enabled || false}
                            onChange={() => setShowVerify(true)}>{''}</Switch>
                    {missingDependencies.length > 0 && "Missing dependencies"}
                </EnableFeatureBox>
            </div>
        </FeatureStatusContainer>
    )
}
export default FeatureStatus