import * as React from 'react'
import styled from 'styled-components'
import {FeaturesQuery, useConfigurationQuery} from '../../lib/schema/graphql'
import LoaderSpinner from '../lib/spinner'
import ErrorMessage from '../lib/error'
import {Success} from "@navikt/ds-icons";
import {navGronn} from "../../styles/constants";
import ConfigPage from "./configPage";


const FeatureContainer = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 5px;
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
  font-family: monospace;
`

interface FeatureProps {
    feature?: FeaturesQuery['features'][0],
}


const Feature = ({feature}: FeatureProps) => {
    if (!feature) {
        return <EmptyFeature/>
    }

    return (
        <FeatureContainer>
            <FeatureStatus>
                <div key={feature.name} style={{display: 'flex', flexDirection: 'column'}}>
                    {feature.chart && <div>chart: {feature.chart}</div>}
                    {feature.repo && <div>repo: {feature.repo}</div>}
                    {feature.source && <div>source: {feature.source}</div>}
                    {feature.version && <div>version: {feature.version}</div>}
                    {feature.dependsOn.length > 0 && <div>dependencies: {feature.dependsOn.join(", ")}</div>}
                    {feature.environmentKinds && <div>environment kinds: {feature.environmentKinds.map(s => s.toLowerCase()).join(", ")}</div>}
                </div>
            </FeatureStatus>
            <ConfigPage feature={feature} />

        </FeatureContainer>
)
}
export default Feature
