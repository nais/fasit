import gql from 'graphql-tag'

export const CONFIGURATION_CREATE = gql`
  mutation configurationCreate(
    $description: String
    $feature: String!
    $key: String!
    $value: RawMessage!
    $environmentID: ID
  ) {
    configurationCreate(
      configuration: {
        feature: $feature
        description: $description
        key: $key
        value: $value
        environmentID: $environmentID
      }
    ) {
      id
      key
    }
  }
`
