// noinspection JSUnusedGlobalSymbols

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
      values {
        key
        value
      }
      featureStates {
        enabled
        lastModified
        created
        rolloutStatus
        feature {
          name
          version
          chart
          dependsOn {
            anyOf
            allOf
          }
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
      values {
        key
        value
      }
      featureStates {
        enabled
        lastModified
        created
        rolloutStatus
        feature {
          name
          version
          chart
          repo
          source
          config
          dependsOn {
            anyOf
            allOf
          }
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
      nodes {
        name
        kubeletVersion
        internalIP
        conditions {
          type
          status
        }
      }
    }
  }
`
