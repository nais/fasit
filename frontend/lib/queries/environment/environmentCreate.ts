import gql from 'graphql-tag'

export const ENVIRONMENT_CREATE = gql`
  mutation environmentCreate(
    $name: String!
    $description: String
    $partnerID: ID!
    $kind: EnvironmentKind!
  ) {
    environmentCreate(
      environment: {
        name: $name
        description: $description
        partnerID: $partnerID
        kind: $kind
      }
    ) {
      id
    }
  }
`
