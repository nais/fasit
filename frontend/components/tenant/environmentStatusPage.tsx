import * as React from 'react'
import styled from 'styled-components'
import {
  ConditionStatus,
  EnvironmentGetByNamesQuery,
  EnvironmentKind,
  KubernetesNodeConditionType,
  useEnvironmentGetReportQuery,
  useUserInfoQuery,
} from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import humanizeDate from '../lib/humanizeDate'
import { Table, Tabs } from '@navikt/ds-react'
import ReportStatus from './reportStatus'
import StatusCircle from '../lib/statusCircle'
import {
  navGronn,
  navMorkGra,
  navRod,
  redError,
  redErrorLighten80,
} from '../../styles/constants'
import FeatureValues from './featureValues'
import { parseISO } from 'date-fns'
import Link from 'next/link'
import { useRouter } from 'next/router'

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
  const { data, loading, error } = useEnvironmentGetReportQuery({
    variables: { id: env.id },
    pollInterval: 10 * 1000,
  })
  const infoQuery = useUserInfoQuery()

  if (error) return <ErrorMessage error={error} />
  if (loading || !data) return <LoaderSpinner />

  const email = infoQuery?.data?.userInfo?.email
  const report = data.environment
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
      <ReportStatus reportedAt={report.health.reportedAt} />
      <br />
      <a
        href={`https://console.cloud.google.com/welcome?project=${getValue(
          'project_id',
        )}&authuser=${email}`}
      >
        Console
      </a>
      {parseISO(report.health.reportedAt).getFullYear() === 1969 && (
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
          router.push(router)
        }}
      >
        <Tabs.List>
          <Tabs.Tab value="releases" label="Releases" />
          <Tabs.Tab value="env_values" label="Environment values" />
          <Tabs.Tab value="nodes" label="Kubernetes nodes" />
        </Tabs.List>
        <Tabs.Panel value="releases" className="h-24 w-full bg-gray-50 p-8">
          <>{warnings.map((w, i) => warningLink(tenantName, env.name, w, i))}</>
          <h3>Helm installs</h3>
          <Table size={'small'}>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>name</Table.HeaderCell>
                <Table.HeaderCell>status</Table.HeaderCell>
                <Table.HeaderCell>version</Table.HeaderCell>
                <Table.HeaderCell>last deployed</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {report.releases.map((r) => (
                <Table.Row
                  key={r.name}
                  style={r.feature ? {} : { backgroundColor: '#ffd5d5' }}
                >
                  <Table.DataCell>{r.name}</Table.DataCell>
                  <Table.DataCell>{r.status}</Table.DataCell>
                  <Table.DataCell>{r.version}</Table.DataCell>
                  <Table.DataCell>
                    {humanizeDate(r.lastDeployed, '', true)}
                  </Table.DataCell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </Tabs.Panel>

        <Tabs.Panel value="env_values" className="h-24  w-full bg-gray-50 p-8">
          <h3>Environment values</h3>
          <FeatureValues values={env.values} />
        </Tabs.Panel>

        <Tabs.Panel value="nodes" className="h-24 w-full bg-gray-50 p-8">
          <h3>Kubernetes nodes</h3>
          <Table size={'small'}>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>name</Table.HeaderCell>
                <Table.HeaderCell>status</Table.HeaderCell>
                <Table.HeaderCell>internal ip</Table.HeaderCell>
                <Table.HeaderCell>version</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {report.nodes.map((r) => (
                <Table.Row key={r.name}>
                  <Table.DataCell>{r.name}</Table.DataCell>
                  <Table.DataCell>
                    {r.conditions.find((c) => {
                      return c.type === KubernetesNodeConditionType.Ready
                    })?.status === ConditionStatus.True ? (
                      <>
                        <StatusCircle color={navGronn} /> Ready{' '}
                      </>
                    ) : (
                      <>
                        <StatusCircle color={navRod} /> NotReady{' '}
                      </>
                    )}
                  </Table.DataCell>
                  <Table.DataCell>{r.internalIP}</Table.DataCell>
                  <Table.DataCell>{r.kubeletVersion}</Table.DataCell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </Tabs.Panel>
      </Tabs>
    </EnvironmentStatus>
  )
}
export default EnvironmentStatusPage
