import ErrorMessage from './error'
import { useEnvironmentAuditLogQuery } from '../../lib/schema/graphql'
import LoaderSpinner from './spinner'
import humanizeDate from './humanizeDate'
import styled from 'styled-components'

interface AuditViewParams {
  envID: string
  featureName?: string
}

const AuditLine = styled.div`
  padding: 5px;

  &:nth-child(even) {
    background-color: #f5f5f5;
  }

  &:not(:last-child) {
    border-bottom: 1px solid silver;
  }

  .details {
    font-size: 0.8em;
    color: #666;
  }
`

const AuditView = ({ envID, featureName }: AuditViewParams) => {
  var { data, error, loading } = useEnvironmentAuditLogQuery({
    variables: { envID, featureName },
  })

  if (error) {
    return <ErrorMessage error={error} />
  }
  if (loading || !data) {
    return <LoaderSpinner />
  }

  return (
    <>
      {data.environment.auditLog.map((e, i) => (
        <AuditLine key={i}>
          {e.description}
          <div className="details">
            {e.actor} @ {humanizeDate(e.createdAt, 'dd. MMMM yyyy HH:mm:ss')}
          </div>
        </AuditLine>
      ))}
    </>
  )
}
export default AuditView
