import gql from 'graphql-tag'

export const TENANTS_GET = gql`
  query TenantsGet {
    tenants {
      id
      name
      description
      created
      lastModified
    }
  }
`
