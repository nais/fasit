import gql from 'graphql-tag'

export const TENANT_CREATE = gql`
  mutation tenantCreate($name: String!, $description: String) {
    tenantCreate(tenant: { name: $name, description: $description }) {
      id
    }
  }
`
