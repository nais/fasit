import { Table, Tabs } from '@navikt/ds-react'
import { parseISO } from 'date-fns'
import Link from 'next/link'
import { useRouter } from 'next/router'
import styled from 'styled-components'
import {
  ConditionStatus,
  EnvironmentGetByNamesQuery,
  KubernetesNodeConditionType,
  useEnvironmentGetReportQuery,
  useUserInfoQuery,
} from '../../lib/schema/graphql'
import {
  navGronn,
  navMorkGra,
  navRod,
  redError,
  redErrorLighten80,
} from '../../styles/constants'
import AuditView from '../lib/auditView'
import ErrorMessage from '../lib/error'
import humanizeDate from '../lib/humanizeDate'
import LoaderSpinner from '../lib/spinner'
import StatusCircle from '../lib/statusCircle'
import FeatureValues from './featureValues'
import HelmInstalls from './helmInstalls'
import KubernetesNodes from './kubernetesNodes'
import ReportStatus from './reportStatus'

const WarningLink = styled.a`
  display: block;
  color: ${navMorkGra};
  cursor: pointer;
  background-color: #fde8e6;
  border: 1px solid ${redError};
  border-radius: 5px;
  padding: 0.5em 1em;
  margin: 5px;
  :hover {
    background-color: ${redErrorLighten80};
  }
`

const EnvironmentStatus = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
  border-left: 1px solid silver;
`

interface EnvironmentStatusPageProps {
  env: EnvironmentGetByNamesQuery['environmentByNames']
  tenantName: string
}

const warningLink = (
  tenantName: string,
  environmentName: string,
  w: EnvironmentGetByNamesQuery['environmentByNames']['warnings'][0],
  i: number,
) => {
  if (w.__typename === 'FeatureWarning') {
    return (
      <Link
        href={`/tenant/${tenantName}/${environmentName}?feature=${w.feature.name}`}
        key={i}
      >
        <WarningLink>
          <strong>{w.feature.name}</strong>: {w.message}
        </WarningLink>
      </Link>
    )
  }
  return (
    <Link key={i} href="#">
      <WarningLink key={i}>{w.message}</WarningLink>
    </Link>
  )
}

const EnvironmentStatusPage = ({
  env,
  tenantName,
}: EnvironmentStatusPageProps) => {
  const router = useRouter()
  const infoQuery = useUserInfoQuery()

  const email = infoQuery?.data?.userInfo?.email
  const warnings = env.warnings
  const getValue = (key: string) => {
    return env.values.find((v) => v.key === key)?.value
  }

  let activeTab = router.query.tab as string
  if (!activeTab) {
    activeTab = 'releases'
  }

  return (
    <EnvironmentStatus>
      <ReportStatus reportedAt={env.health.reportedAt} />
      <br />
      <a
        href={`https://console.cloud.google.com/welcome?project=${getValue(
          'project_id',
        )}&authuser=${email}`}
      >
        Console
      </a>
      {parseISO(env.health.reportedAt).getFullYear() === 1969 && (
        <>
          <h3>
            naisd not installed.{' '}
            <Link
              href={`/tenant/${tenantName}/${env.name}?feature=naisd&tab=helm_values`}
            >
              <a>More info here.</a>
            </Link>
          </h3>
        </>
      )}

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
        <>{warnings.map((w, i) => warningLink(tenantName, env.name, w, i))}</>
        <Tabs.List>
          <Tabs.Tab value="releases" label="Releases" />
          <Tabs.Tab value="env_values" label="Environment values" />
          <Tabs.Tab value="nodes" label="Kubernetes nodes" />
          <Tabs.Tab value="audit_log" label="Audit log" />
        </Tabs.List>
        <Tabs.Panel value="releases" className="h-24 w-full bg-gray-50 p-8">
          <HelmInstalls envID={env.id} />
        </Tabs.Panel>

        <Tabs.Panel value="env_values" className="h-24  w-full bg-gray-50 p-8">
          <h3>Environment values</h3>
          <FeatureValues
            values={[...env.values].sort((a, b) => {
              return a.key.localeCompare(b.key)
            })}
          />
        </Tabs.Panel>

        <Tabs.Panel value="nodes" className="h-24 w-full bg-gray-50 p-8">
          <KubernetesNodes envID={env.id} />
        </Tabs.Panel>

        <Tabs.Panel value="audit_log" className="h-24 w-full bg-gray-50 p-8">
          <AuditView envID={env.id} />
        </Tabs.Panel>
      </Tabs>
    </EnvironmentStatus>
  )
}
export default EnvironmentStatusPage
