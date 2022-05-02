import * as React from 'react'
import {useEffect, useState} from 'react'
import styled from 'styled-components'
import ConfigPage from './configPage'
import {EnvironmentGetQuery, useStatusForFeatureSubscription} from '../../lib/schema/graphql'
import {navGronn, navRod} from '../../styles/constants'
import IconBox from "../lib/icons/iconBox";
import GitIcon from "../lib/icons/gitIcon";
import {Loader, Switch} from '@navikt/ds-react'
import EnableFeature from "./enableFeature";
import humanizeDate from "../lib/humanizeDate";


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

interface FeatureProps {
    env: EnvironmentGetQuery['environment']
    featureName: string,
}


const Feature = ({env, featureName}: FeatureProps) => {
    const featureState = env.featureStates.find((f) => f.feature.name === featureName)
    if (!env || !featureState) {
        return <EmptyFeature/>
    }
    const f = featureState.feature
    const [showVerify, setShowVerify] = useState(false)
    const missingDependencies = f.dependsOn.filter((dependency) => {
        return !env.featureStates.find((fs) => fs.feature.name === dependency)?.enabled
    })
    const {loading, error, data} = useStatusForFeatureSubscription({variables: {envID: env.id, feature: featureName}})

    // dirty, dirty hack to reload page when subscription is cancelled by HMR in dev
    useEffect(() => {
        if (error?.message === "Observable cancelled prematurely") {
            window.location.reload()
        }
    }, [error])

    return (
        <FeatureContainer>
            <FeatureStatus>
                <div style={{display: 'flex', alignItems: 'center'}} key={f.name}>
                    <div key={f.name} style={{display: 'flex', flexDirection: 'column', flexGrow: '1'}}>
                        {f.chart && <div>chart: {f.chart}</div>}
                        {f.repo && <div>repos: {f.repo}</div>}
                        {f.version && <div>version: {f.version}</div>}
                        {f.source && <div style={{display: 'flex', width: 'fit-content', gap: '10px'}}><IconBox
                            size={20}><GitIcon/></IconBox> <a href={f.source} target="_blank">{f.source}</a></div>}
                        {f.dependsOn.length > 0 && <div>dependencies: {f.dependsOn.map((d) => {
                            return <span key={d} style={{color: missingDependencies.includes(d) ? navRod : navGronn}}>{d + " "}</span>
                        })}
                        </div>
                        }
                        <div>

                        {loading || !data && <Loader transparent /> }
                        <div>status: {data?.status.status}</div>
                        <div>version: {data?.status.version}</div>
                        <div>lastModified: {humanizeDate(data?.status.lastModified)}</div>
                        <div>created: {humanizeDate(data?.status.created)}</div>
                        </div>
                    </div>
                    <EnableFeatureBox>
                        <div>Enabled</div>
                        <Switch disabled={missingDependencies.length > 0} size="medium" checked={featureState.enabled} onChange={() => setShowVerify(true)}>{''}</Switch>
                        {missingDependencies.length > 0 && "Missing dependencies"}
                    </EnableFeatureBox>
                </div>
            </FeatureStatus>
            {
                featureName && env && <ConfigPage env={env} feature={featureName}/>
            }
            <EnableFeature open={showVerify} onClose={setShowVerify} feature={f.name} envID={env.id} enabled={featureState.enabled}/>
        </FeatureContainer>
    )
}
export default Feature