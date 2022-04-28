import gql from 'graphql-tag'

export const TENANT_GET = gql`
  query TenantGet {
    tenant {
      id
      name
      description
      created
      lastModified
    }
  }
`
