import { Tabs } from '@navikt/ds-react'
import { useRouter } from 'next/router'
import * as React from 'react'
import styled from 'styled-components'
import { useFeatureDetailsQuery } from '../../lib/schema/graphql'
import ConfigPage from './configPage'
import Link from 'next/link'
import humanizeDate from '../lib/humanizeDate'
import { rolloutStatus } from '../rollout/rollout'
import ErrorMessage from '../../components/lib/error'
import LoaderSpinner from '../../components/lib/spinner'

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

const OverrideKeys = styled.pre`
  font-size: 0.8em;
  margin-top: 0;
  margin-bottom: 10px;
`

const WarningBox = styled.div`
  border: 1px solid #f5a623;
  border-radius: 5px;
  padding: 10px;
  background-color: #fff8e6;
  font-size: 0.8em;
  margin-bottom: 10px;
`

interface FeatureProps {
  featureName: string
}

const Feature = ({ featureName }: FeatureProps) => {
  const router = useRouter()
  const { data, error, loading } = useFeatureDetailsQuery({
    variables: { name: featureName },
  })

  if (loading || !data) {
    return <LoaderSpinner />
  }
  if (error) {
    return <ErrorMessage error={error} />
  }

  const feature = data.feature

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
          {feature.outdatedInfo.length > 0 && (
            <WarningBox>
              {feature.outdatedInfo.map((s, i) => {
                if (s?.dependency) {
                  return (
                    <div key={i}>
                      Outdated dependency: {s.dependencyName} has latest version{' '}
                      <strong>{s.newVersion}</strong>
                    </div>
                  )
                }
                return (
                  <div key={i}>
                    Outdated: latest version is <strong>{s?.newVersion}</strong>
                  </div>
                )
              })}
            </WarningBox>
          )}
          {feature.description && <div>description: {' '}{feature.description}</div>}
        </div>
      </FeatureStatus>

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
          <Tabs.Tab value="config" label="Config" />
          <Tabs.Tab
            value="rollouts"
            label={`Rollouts (${feature.rolloutSummaries.length})`}
          />
          <Tabs.Tab
            value="overrides"
            label={`Overrides (${feature.configoverrides.length})`}
          />
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
          {feature.configoverrides
            ?.concat()
            .sort((a, b) =>
              a.environment?.tenant?.name.localeCompare(
                b.environment?.tenant?.name,
              ),
            )
            .map((e, i) => (
              <div key={i}>
                <Link
                  href={`/tenant/${e.environment.tenant.name}/${e.environment.name}?feature=${feature.name}`}
                >
                  <a
                    href={`/tenant/${e.environment.tenant.name}/${e.environment.name}?feature=${feature.name}`}
                  >
                    {e.environment.tenant.name} - {e.environment.name}
                  </a>
                </Link>
                <OverrideKeys>{e.keys.join('\n')}</OverrideKeys>
              </div>
            ))}
        </Tabs.Panel>
      </Tabs>
    </FeatureContainer>
  )
}
export default Feature
