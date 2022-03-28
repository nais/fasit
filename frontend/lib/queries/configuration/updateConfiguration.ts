import gql from 'graphql-tag'

export const CONFIGURATION_UPDATE = gql`
  mutation configurationUpdate(
    $description: String
    $id: ID!
    $value: RawMessage!
  ) {
    configurationUpdate(
      id: $id
      configuration: { description: $description, value: $value }
    ) {
      id
      key
    }
  }
`
