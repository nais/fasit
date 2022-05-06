import * as React from 'react'
import styled from 'styled-components'
import {useEnvironmentGetReportQuery} from '../../lib/schema/graphql'
import ErrorMessage from "../lib/error";
import LoaderSpinner from "../lib/spinner";
import humanizeDate from "../lib/humanizeDate";
import {Table} from "@navikt/ds-react";
import ReportStatus from "./reportStatus";


const EnvironmentStatus = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 0px;
  border-left: 1px solid silver;
`
interface  EnvironmentStatusPageProps {
    environmentID: string
}
const EnvironmentStatusPage = ({environmentID}: EnvironmentStatusPageProps) => {
    const {data, loading, error} = useEnvironmentGetReportQuery({variables: {id: environmentID}, pollInterval: 10 * 1000})
    if (error) return <ErrorMessage error={error}/>
    if (loading || !data)return  <LoaderSpinner/>
    const report = data.environment

    return (
        <EnvironmentStatus>
            <ReportStatus reportedAt={report.health.reportedAt}/>
            <h3>Kubernetes nodes</h3>
            <Table size={"small"}>
                <Table.Header>
                    <Table.Row>
                        <Table.HeaderCell>name</Table.HeaderCell>
                        <Table.HeaderCell>status</Table.HeaderCell>
                        <Table.HeaderCell>version</Table.HeaderCell>
                    </Table.Row>
                </Table.Header>
                <Table.Body>
                    {report.nodes.map((r) => (
                        <Table.Row key={r.name}>
                            <Table.DataCell>{r.name}</Table.DataCell>
                            <Table.DataCell>{r.phase}</Table.DataCell>
                            <Table.DataCell>{r.kubeletVersion}</Table.DataCell>
                        </Table.Row>))}

                </Table.Body>
            </Table>
            <h3>Helm installs</h3>
            <Table size={"small"}>
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
                            <Table.DataCell>{humanizeDate(r.lastDeployed, "", true)}</Table.DataCell>
                            </Table.Row>))}

                </Table.Body>
            </Table>
        </EnvironmentStatus>
    )
}
export default EnvironmentStatusPage