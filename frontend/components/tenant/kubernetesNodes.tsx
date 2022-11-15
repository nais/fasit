import { Table } from '@navikt/ds-react'
import { useEffect } from 'react'
import {
  ConditionStatus,
  KubernetesNodeConditionType,
  useEnvironmentKubernetesNodesQuery,
} from '../../lib/schema/graphql'
import { useFocusPoll } from '../../lib/useFocusPoll'
import { navGronn, navRod } from '../../styles/constants'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import StatusCircle from '../lib/statusCircle'

interface KubernetesNodesProps {
  envID: string
}

const KubernetesNodes = ({ envID }: KubernetesNodesProps) => {
  const query = useEnvironmentKubernetesNodesQuery({
    variables: { id: envID },
  })

  useFocusPoll({ pollInterval: 10 * 1000, ...query })

  const { data, loading, error } = query

  if (error) return <ErrorMessage error={error} />
  if (loading || !data) return <LoaderSpinner />

  return (
    <>
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
          {data.environment.nodes.map((r) => (
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
    </>
  )
}
export default KubernetesNodes
