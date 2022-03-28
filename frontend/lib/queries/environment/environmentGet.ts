import gql from 'graphql-tag'

export const ENVIRONMENT_GET = gql`
  query environmentGet($id: ID!) {
    environment(id: $id) {
      id
      name
      description
      lastModified
      created
    }
  }
`
