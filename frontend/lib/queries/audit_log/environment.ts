import gql from 'graphql-tag'

export const ENVIRONMENT_AUDIT_LOG = gql`
  query EnvironmentAuditLog($envID: ID!, $featureName: String) {
    environment(id: $envID) {
      id
      auditLog(featureName: $featureName) {
        actor
        description
        objectId
        objectType
        createdAt
      }
    }
  }
`
