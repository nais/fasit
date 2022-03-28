import gql from 'graphql-tag'

export const PARTNERS_GET = gql`
  query PartnersGet {
    partners {
      id
      name
      description
      created
      lastModified
    }
  }
`
