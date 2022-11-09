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
export const TENANT_GET_BY_NAME = gql`
  query TenantGetByName($slug: String!) {
    tenant(slug: $slug) {
      id
      name
      description
      environments {
        id
        name
        warnings {
          message
        }
      }
      created
      lastModified
      warnings {
        message

        ... on FeatureWarning {
          feature {
            name
          }
          environment {
            name
          }
        }

        ... on NaisdWarning {
          environment {
            name
          }
        }
      }
    }
  }
`
