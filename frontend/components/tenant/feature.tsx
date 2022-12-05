import { FileContent, Filter, Notes, Wrench } from '@navikt/ds-icons'
import { Tabs } from '@navikt/ds-react'
import { useRouter } from 'next/router'
import { useEffect, useState } from 'react'
import styled from 'styled-components'
import { useFeatureStateQuery } from '../../lib/schema/graphql'
import { useFocusPoll } from '../../lib/useFocusPoll'
import AuditView from '../lib/auditView'
import EnableFeature from './enableFeature'
import FeatureConfig from './featureConfig'
import FeatureHelmValues from './featureHelmValues'
import FeatureLogs from './featureLogs'
import FeatureStatus from './featureStatus'
import RedeployFeature from './redeployFeature'

const FeatureContainer = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
`

interface FeatureProps {
  envID: string
  featureName: string
}

const Feature = ({ envID, featureName }: FeatureProps) => {
  const [showVerify, setShowVerify] = useState(false)
  const [showRedeploy, setShowRedeploy] = useState(false)

  const router = useRouter()

  const query = useFeatureStateQuery({
    variables: { envID: envID, feature: featureName },
  })
  useFocusPoll({ pollInterval: 10 * 1000, ...query })

  const { data, loading } = query

  let activeTab = router.query.tab as string
  if (!activeTab) {
    activeTab = 'config'
  }

  if (loading && !data) {
    return <div>Loading...</div>
  }

  if (!data) {
    return <div>Failed loading...</div>
  }

  const featureState = data.featureState

  return (
    <FeatureContainer>
      <FeatureStatus
        featureState={featureState}
        setShowVerify={setShowVerify}
        setShowRedeploy={setShowRedeploy}
      />
      <Tabs
        defaultValue={activeTab}
        size="small"
        iconPosition="left"
        onChange={(value) => {
          router.query.tab = value
          router.push({
            pathname: router.pathname,
            query: router.query,
          })
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
            value="audit_log"
            label="Audit log"
            icon={<Notes title="audit log" />}
          />
        </Tabs.List>
        <Tabs.Panel value="config" className="h-24 w-full bg-gray-50 p-8">
          <FeatureConfig
            featureState={featureState}
            envID={envID}
            key={featureName}
          />
        </Tabs.Panel>
        <Tabs.Panel value="logs" className="h-24 w-full bg-gray-50 p-8">
          <FeatureLogs envID={envID} featureName={featureName} />
        </Tabs.Panel>
        <Tabs.Panel value="helm_values" className="h-24  w-full bg-gray-50 p-8">
          <FeatureHelmValues envID={envID} feature={featureState.feature} />
        </Tabs.Panel>
        <Tabs.Panel value="audit_log" className="h-24  w-full bg-gray-50 p-8">
          <AuditView envID={envID} featureName={featureName} />
        </Tabs.Panel>
      </Tabs>

      <EnableFeature
        open={showVerify}
        onClose={setShowVerify}
        feature={featureName}
        envID={envID}
        enabled={featureState.enabled}
      />
      <RedeployFeature
        open={showRedeploy}
        onClose={setShowRedeploy}
        feature={featureName}
        envID={envID}
      />
    </FeatureContainer>
  )
}
export default Feature
