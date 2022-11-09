import gql from 'graphql-tag'

export const TENANTS_GET = gql`
  query TenantsGet {
    tenants {
      id
      name
      description
      created
      lastModified
      warnings {
        message
      }
    }
    outdatedInfo {
      dependency # Just a random field to get list length
    }
  }
`
