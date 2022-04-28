import gql from 'graphql-tag'

export const ENVIRONMENTS_GET = gql`
  query environmentsGet($tenantID: ID!) {
    environments(tenantID: $tenantID) {
      id
      name
    }
  }
`
