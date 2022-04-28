import gql from 'graphql-tag'

export const ENVIRONMENT_CREATE = gql`
  mutation environmentCreate(
    $name: String!
    $description: String
    $tenantID: ID!
    $kind: EnvironmentKind!
  ) {
    environmentCreate(
      environment: {
        name: $name
        description: $description
        tenantID: $tenantID
        kind: $kind
      }
    ) {
      id
    }
  }
`
