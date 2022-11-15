import { Switch } from '@navikt/ds-react'
import Link from 'next/link'
import { useRouter } from 'next/router'
import * as React from 'react'
import ReactTooltip from 'react-tooltip'
import styled from 'styled-components'
import { FeatureStateQuery, RolloutStatus } from '../../lib/schema/graphql'
import { navBla, navGronn, navOransje, navRod } from '../../styles/constants'
import GitIcon from '../lib/icons/gitIcon'
import IconBox from '../lib/icons/iconBox'

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

const FlatButton = styled.button`
  background-color: transparent;
  border: none;
  color: ${navBla};
  cursor: pointer;
  padding: 0;
  text-decoration: underline;

  &:hover {
    text-decoration: none;
  }
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
  }}
`
const StatusField = styled.div`
  display: flex;
  align-items: center;
`

interface FeatureProps {
  featureState: FeatureStateQuery['featureState']
  setShowVerify: React.Dispatch<boolean>
  setShowRedeploy: React.Dispatch<boolean>
}

const FeatureStatus = ({
  featureState,
  setShowVerify,
  setShowRedeploy,
}: FeatureProps) => {
  const router = useRouter()
  const tenantName = router.query.tenantName as string
  const envName = (router.query.environmentName as string[])[0]

  const {
    feature,
    configuration,
    rolloutStatus: status,
    missingDependencies,
  } = featureState

  const requiredConfigs = configuration.configuration
    .filter((c) => c.required)
    .sort()

  const missingRequirements = requiredConfigs.filter(
    (r) => r.required && !r.value,
  )

  const dependencies = feature.dependsOn
    .map((a) => a.allOf.concat(a.anyOf))
    .flat()

  return (
    <FeatureStatusContainer>
      <div style={{ display: 'flex', alignItems: 'center' }} key={feature.name}>
        <div
          key={feature.name}
          style={{ display: 'flex', flexDirection: 'column', flexGrow: '1' }}
        >
          <div>
            <StatusField>
              status: <StatusIndicator status={status} />
              {status.toLowerCase()}
            </StatusField>
          </div>
          {feature.chart && <div>chart: {feature.chart}</div>}
          {feature.repo && <div>repo: {feature.repo}</div>}
          {feature.version && <div>version: {feature.version}</div>}
          {feature.source && (
            <div style={{ display: 'flex', width: 'fit-content', gap: '10px' }}>
              <IconBox size={20}>
                <GitIcon />
              </IconBox>{' '}
              <a href={feature.source} target="_blank" rel="noreferrer">
                {feature.source}
              </a>
            </div>
          )}
          {dependencies.length > 0 && (
            <div>
              dependencies:{' '}
              {dependencies.map((d) => {
                return (
                  <Link
                    href={`/tenant/${tenantName}/${envName}?feature=${d}`}
                    key={d}
                  >
                    <a
                      style={{
                        color: missingDependencies.find((a) => a.name === d)
                          ? navRod
                          : navGronn,
                      }}
                    >
                      {d + ' '}
                    </a>
                  </Link>
                )
              })}
            </div>
          )}
        </div>
        <EnableFeatureBox>
          <div>Enabled</div>
          <Switch
            data-tip
            data-for="enable_feature"
            disabled={
              (missingDependencies.length || 0) > 0 ||
              missingRequirements.length > 0
            }
            size="medium"
            checked={featureState?.enabled || false}
            onChange={() => setShowVerify(true)}
          >
            {''}
          </Switch>
          {(missingDependencies.length || 0) > 0 && 'Missing dependencies'}
          <ReactTooltip
            id="enable_feature"
            place="bottom"
            type="info"
            effect="solid"
          >
            {featureState?.enabled ? 'Disable' : 'Enable'} reconciling this
            feature
          </ReactTooltip>
          {featureState?.enabled && (
            <FlatButton onClick={() => setShowRedeploy(true)}>
              Redeploy
            </FlatButton>
          )}
        </EnableFeatureBox>
      </div>
    </FeatureStatusContainer>
  )
}
export default FeatureStatus
