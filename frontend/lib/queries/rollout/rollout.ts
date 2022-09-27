import gql from 'graphql-tag'

export const ROLLOUT = gql`
  query rollout($id: ID!) {
    rollout(id: $id) {
      id
      created
      status
      feature {
        name
      }
      events {
        id
        type
        data
        created
      }
      changeset {
        new
      }
    }
  }
`
