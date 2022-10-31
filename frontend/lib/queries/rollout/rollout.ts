import gql from 'graphql-tag'

export const ROLLOUT_SUMMARY = gql`
  query rolloutSummary($id: ID!) {
    rolloutSummary(id: $id) {
      id
      status
      created
      feature {
        name
      }
      rollouts {
        id
        created
        status
        environment {
          kind
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
  }
`
