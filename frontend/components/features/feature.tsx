import { Tabs } from '@navikt/ds-react'
import { useRouter } from 'next/router'
import * as React from 'react'
import styled from 'styled-components'
import { FeatureDetailsQuery } from '../../lib/schema/graphql'
import ConfigPage from './configPage'
import Link from 'next/link'
import humanizeDate from '../lib/humanizeDate'
import { rolloutStatus } from '../rollout/rollout'

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

const RolloutLink = styled.a`
  display: block;
  margin-bottom: 10px;
`

interface FeatureProps {
  feature?: FeatureDetailsQuery['features'][0]
}

const Feature = ({ feature }: FeatureProps) => {
  const router = useRouter()
  if (!feature) {
    return <EmptyFeature />
  }

  const dependsOn = feature.dependsOn
    ?.map((d) => d.anyOf.concat(d.allOf))
    .flat()

  let activeTab = router.query.tab as string
  if (!activeTab) {
    activeTab = 'config'
  }

  return (
    <FeatureContainer>
      <FeatureStatus>
        <div
          key={feature.name}
          style={{ display: 'flex', flexDirection: 'column' }}
        >
          {feature.chart && <div>chart: {feature.chart}</div>}
          {feature.repo && <div>repo: {feature.repo}</div>}
          {feature.source && <div>source: {feature.source}</div>}
          {feature.version && <div>version: {feature.version}</div>}
          {dependsOn.length > 0 && (
            <div>dependencies: {dependsOn.join(', ')}</div>
          )}
          {feature.environmentKinds && (
            <div>
              environment kinds:{' '}
              {feature.environmentKinds.map((s) => s.toLowerCase()).join(', ')}
            </div>
          )}
        </div>
      </FeatureStatus>

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
          <Tabs.Tab value="config" label="Config" />
          <Tabs.Tab value="rollouts" label="Rollouts" />
          <Tabs.Tab value="overrides" label="Overrides" />
        </Tabs.List>

        <Tabs.Panel value="config" className="h-24 w-full bg-gray-50 p-8">
          <ConfigPage feature={feature} />
        </Tabs.Panel>
        <Tabs.Panel value="rollouts" className="h-24 w-full bg-gray-50 p-8">
          <h3>Rollouts</h3>
          {feature.rolloutSummaries?.map((r, i) => (
            <Link key={i} href={`/rollout/${r.id}`}>
              <RolloutLink href={`/rollout/${r.id}`}>
                {rolloutStatus(r.status)}{' '}
                {humanizeDate(r.created, 'PPPP', true)}
              </RolloutLink>
            </Link>
          ))}
        </Tabs.Panel>
        <Tabs.Panel value="overrides" className="h-24 w-full bg-gray-50 p-8">
          <h3>Environments with overrides</h3>
        </Tabs.Panel>
      </Tabs>
    </FeatureContainer>
  )
}
export default Feature
