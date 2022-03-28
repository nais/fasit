import gql from 'graphql-tag'

export const PARTNER_GET = gql`
  query PartnerGet($id: ID!) {
    partner(id: $id) {
      id
      name
      description
      created
      lastModified
    }
  }
`
