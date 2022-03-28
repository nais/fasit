import gql from 'graphql-tag'

export const ENVIRONMENT_UPDATE = gql`
  mutation environmentUpdate($description: String, $id: ID!) {
    environmentUpdate(id: $id, input: { description: $description }) {
      id
    }
  }
`
