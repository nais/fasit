import gql from 'graphql-tag'

export const ENVIRONMENT_GET_BY_NAMES = gql`
  query environmentGetByNames($tenantName: String!, $environmentName: String!) {
    environmentByNames(
      tenantName: $tenantName
      environmentName: $environmentName
    ) {
      id
      name
      description
      lastModified
      created
      kind
      featureStates {
        enabled
        lastModified
        created
        feature {
          name
          version
          chart
          dependsOn
          repo
          source
          config
        }
      }
    }
  }
`

export const ENVIRONMENT_GET = gql`
  query environmentGet($id: ID!) {
    environment(id: $id) {
      id
      name
      description
      lastModified
      created
      kind
      featureStates {
        enabled
        lastModified
        created
        feature {
          name
          version
          chart
          dependsOn
          repo
          source
          config
        }
      }
    }
  }
`

export const ENVIRONMENT_GET_REPORT = gql`
  query environmentGetReport($id: ID!) {
    environment(id: $id) {
      health {
        reportedAt
      }
      releases {
        feature {
          name
        }
        status
        lastDeployed
        version
      }
    }
  }
`
