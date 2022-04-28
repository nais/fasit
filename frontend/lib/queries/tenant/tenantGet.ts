import gql from 'graphql-tag'

export const TENANT_GET = gql`
  query TenantGet($id: ID!) {
    tenant(id: $id) {
      id
      name
      description
      created
      lastModified
    }
  }
`
