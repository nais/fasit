import gql from 'graphql-tag'

export const TENANTS_LIST = gql`
  query TenantsList {
    tenants {
      name
      warnings {
        message
      }
    }
    outdatedInfo {
      dependency # Just a random field to get list length
    }
  }
`
