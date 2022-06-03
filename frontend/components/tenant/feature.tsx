import * as React from 'react'
import {useState} from 'react'
import styled from 'styled-components'
import FeatureConfig from './featureConfig'
import {EnvironmentGetQuery, useConfigurationQuery, useFeaturesQuery} from '../../lib/schema/graphql'
import EnableFeature from "./enableFeature";
import FeatureStatus from "./featureStatus";
import extractConfig from "../lib/extractConfig";


const FeatureContainer = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
`

interface FeatureProps {
    env: EnvironmentGetQuery['environment']
    featureName: string,
}


const Feature = ({env, featureName}: FeatureProps) => {
    const [showVerify, setShowVerify] = useState(false)

    const configQuery = useConfigurationQuery({variables: {envID: env.id, feature: featureName}})
    const features = useFeaturesQuery({variables: {kind: env.kind}})
    const {configs, featureObject} = extractConfig(features, configQuery, featureName);


    return (
        <FeatureContainer>
            <FeatureStatus featureName={featureName} configs={configs} env={env} setShowVerify={setShowVerify}/>
            <FeatureConfig envID={env.id} configs={configs} featureObject={featureObject} mapping={configQuery.data?.configuration.mapping}/>
            <EnableFeature open={showVerify} onClose={setShowVerify} feature={featureName} envID={env.id}
                           enabled={env.featureStates.find((f) => f.feature.name === featureName)?.enabled || false}/>
        </FeatureContainer>
    )
}
export default Feature