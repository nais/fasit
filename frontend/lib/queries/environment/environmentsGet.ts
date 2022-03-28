import gql from 'graphql-tag'

export const ENVIRONMENTS_GET = gql`
  query environmentsGet($partnerID: ID!) {
    environments(partnerID: $partnerID) {
      id
      name
    }
  }
`
