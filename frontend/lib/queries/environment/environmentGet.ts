// noinspection JSUnusedGlobalSymbols

import gql from 'graphql-tag'

export const ENVIRONMENT_GET_BY_NAMES = gql`
  query environmentGetByNames($tenantName: String!, $environmentName: String!) {
    environmentByNames(
      tenantName: $tenantName
      environmentName: $environmentName
    ) {
      id
      kind
      name
      description
      lastModified
      created
      featureStates {
        enabled
        rolloutStatus
        feature {
          name
        }
      }
      health {
        reportedAt
      }
      warnings {
        message
        ... on FeatureWarning {
          feature {
            name
          }
        }
      }
      # Because we have a Google Cloud Console link on the status page, we need to know the project ID
      # to be able to generate the link. We need to use values to get it from the environment.
      values {
        key
        value
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
      id
      health {
        reportedAt
      }
      releases {
        name
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

export const ENVIRONMENT_KUBERNETES_NODES = gql`
  query environmentKubernetesNodes($id: ID!) {
    environment(id: $id) {
      id
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

export const ENVIRONMENT_HELM_INSTALLS = gql`
  query environmentHelmInstalls($id: ID!) {
    environment(id: $id) {
      id
      releases {
        name
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
