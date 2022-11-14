import * as React from 'react'
import { useState } from 'react'
import styled from 'styled-components'
import FeatureConfig from './featureConfig'
import {
  EnvironmentGetQuery,
  useConfigurationQuery,
  useFeaturesQuery,
} from '../../lib/schema/graphql'
import EnableFeature from './enableFeature'
import FeatureStatus from './featureStatus'
import extractConfig from '../lib/extractConfig'
import { Tabs } from '@navikt/ds-react'
import { FileContent, Filter, Notes, Wrench } from '@navikt/ds-icons'
import FeatureLogs from './featureLogs'
import FeatureHelmValues from './featureHelmValues'
import { useRouter } from 'next/router'
import RedeployFeature from './redeployFeature'
import AuditView from '../lib/auditView'

const FeatureContainer = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
`

const LogPre = styled.pre`
  overflow: auto;
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 14px;
`

interface FeatureProps {
  env: EnvironmentGetQuery['environment']
  featureName: string
}

const Feature = ({ env, featureName }: FeatureProps) => {
  const [showVerify, setShowVerify] = useState(false)
  const [showRedeploy, setShowRedeploy] = useState(false)

  const router = useRouter()

  const configQuery = useConfigurationQuery({
    variables: { envID: env.id, feature: featureName },
  })
  const features = useFeaturesQuery({ variables: { kind: env.kind } })
  const { configs, featureObject } = extractConfig(
    features,
    configQuery,
    featureName,
  )

  let activeTab = router.query.tab as string
  if (!activeTab) {
    activeTab = 'config'
  }

  return (
    <FeatureContainer>
      <FeatureStatus
        featureName={featureName}
        configs={configs}
        env={env}
        setShowVerify={setShowVerify}
        setShowRedeploy={setShowRedeploy}
      />
      <Tabs
        defaultValue={activeTab}
        size="small"
        iconPosition="left"
        onChange={(value) => {
          router.query.tab = value
          router.push(router)
        }}
      >
        <Tabs.List>
          <Tabs.Tab
            value="config"
            label="Config"
            icon={<Wrench title="config" />}
          />
          <Tabs.Tab value="logs" label="Logs" icon={<Filter title="logs" />} />
          <Tabs.Tab
            value="helm_values"
            label="Helm Values"
            icon={<FileContent title="helm values" />}
          />
          <Tabs.Tab
            value="audit"
            label="Audit"
            icon={<Notes title="audit log" />}
          />
        </Tabs.List>
        <Tabs.Panel value="config" className="h-24 w-full bg-gray-50 p-8">
          <FeatureConfig
            envID={env.id}
            configs={configs}
            featureObject={featureObject}
            mapping={configQuery.data?.configuration.mapping}
          />
        </Tabs.Panel>
        <Tabs.Panel value="logs" className="h-24 w-full bg-gray-50 p-8">
          <FeatureLogs env={env} featureName={featureName} />
        </Tabs.Panel>
        <Tabs.Panel value="helm_values" className="h-24  w-full bg-gray-50 p-8">
          <FeatureHelmValues env={env} featureName={featureName} />
        </Tabs.Panel>
        <Tabs.Panel value="audit" className="h-24  w-full bg-gray-50 p-8">
          <AuditView envID={env.id} featureName={featureName} />
        </Tabs.Panel>
      </Tabs>

      <EnableFeature
        open={showVerify}
        onClose={setShowVerify}
        feature={featureName}
        envID={env.id}
        enabled={
          env.featureStates.find((f) => f.feature.name === featureName)
            ?.enabled || false
        }
      />
      <RedeployFeature
        open={showRedeploy}
        onClose={setShowRedeploy}
        feature={featureName}
        envID={env.id}
      />
    </FeatureContainer>
  )
}
export default Feature
