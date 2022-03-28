import gql from 'graphql-tag'

export const CONFIGURATION_DELETE = gql`
  mutation configurationDelete($id: ID!) {
    configurationDelete(id: $id)
  }
`
