import * as React from 'react'
import styled from 'styled-components'
import {
    ConditionStatus,
    EnvironmentGetQuery, EnvironmentKind,
    KubernetesNodeConditionType,
    useEnvironmentGetReportQuery, useUserInfoQuery,
} from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import humanizeDate from '../lib/humanizeDate'
import {Table} from '@navikt/ds-react'
import ReportStatus from './reportStatus'
import StatusCircle from '../lib/statusCircle'
import {navGronn, navRod} from '../../styles/constants'
import FeatureValues from './featureValues'
import {parseISO} from "date-fns";

const EnvironmentStatus = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
  border-left: 1px solid silver;
`

interface EnvironmentStatusPageProps {
    env: EnvironmentGetQuery['environment']
    tenantName: string
}

const EnvironmentStatusPage = ({env,tenantName}: EnvironmentStatusPageProps) => {
    const {data, loading, error} = useEnvironmentGetReportQuery({
        variables: {id: env.id},
        pollInterval: 10 * 1000,
    })
    const infoQuery = useUserInfoQuery()

    if (error) return <ErrorMessage error={error}/>
    if (loading || !data) return <LoaderSpinner/>

    const email = infoQuery?.data?.userInfo?.email
    const report = data.environment
    const getValue = (key: string) =>{
        return env.values.find((v) => v.key === key)?.value
    }
    return (
        <EnvironmentStatus>
            <ReportStatus reportedAt={report.health.reportedAt}/>
            <br/>
            <a href={`https://console.cloud.google.com/welcome?project=${getValue('project_id')}&authuser=${email}`}>Console</a>
          {report.health.reportedAt}
            {
                parseISO(report.health.reportedAt).getFullYear() === 1969 &&
                <>
                <h3>Install naisd using the following helm command:</h3>
                <pre style={{fontSize: "14px", padding: '0 8px', backgroundColor: '#f5f5f5', border: '1px solid silver'}}>
                    {`
helm install naisd \\
--namespace "nais-system" \\
--create-namespace \\
--set "tenantName=${tenantName}" \\
--set "env=${env.name}" \\
--set "envProjectId=${getValue('project_id')}" \\
--set "management=${env.kind === EnvironmentKind.Management ? 'true' : 'false'}" \\
oci://europe-north1-docker.pkg.dev/nais-io/nais/naisd
                    `}
                </pre>
                </>
            }
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
                                        <StatusCircle color={navGronn}/> Ready{' '}
                                    </>
                                ) : (
                                    <>
                                        <StatusCircle color={navRod}/> NotReady{' '}
                                    </>
                                )}
                            </Table.DataCell>
                            <Table.DataCell>{r.internalIP}</Table.DataCell>
                            <Table.DataCell>{r.kubeletVersion}</Table.DataCell>
                        </Table.Row>
                    ))}
                </Table.Body>
            </Table>
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
                        <Table.Row key={r.feature.name}>
                            <Table.DataCell>{r.feature.name}</Table.DataCell>
                            <Table.DataCell>{r.status}</Table.DataCell>
                            <Table.DataCell>{r.version}</Table.DataCell>
                            <Table.DataCell>
                                {humanizeDate(r.lastDeployed, '', true)}
                            </Table.DataCell>
                        </Table.Row>
                    ))}
                </Table.Body>
            </Table>
            <h3>Environment values</h3>
            <FeatureValues values={env.values}/>
        </EnvironmentStatus>
    )
}
export default EnvironmentStatusPage
