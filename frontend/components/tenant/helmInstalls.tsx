import { Table } from '@navikt/ds-react'
import { useEnvironmentHelmInstallsQuery } from '../../lib/schema/graphql'
import { useFocusPoll } from '../../lib/useFocusPoll'
import ErrorMessage from '../lib/error'
import humanizeDate from '../lib/humanizeDate'
import LoaderSpinner from '../lib/spinner'

interface HelmInstallsProps {
  envID: string
}

const HelmInstalls = ({ envID }: HelmInstallsProps) => {
  const query = useEnvironmentHelmInstallsQuery({
    variables: { id: envID },
  })

  useFocusPoll({ pollInterval: 10 * 1000, ...query })
  const { data, loading, error } = query
  if (error) return <ErrorMessage error={error} />
  if (loading || !data) return <LoaderSpinner />

  return (
    <>
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
          {data.environment.releases.map((r) => (
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
    </>
  )
}
export default HelmInstalls
