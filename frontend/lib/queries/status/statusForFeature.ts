import gql from 'graphql-tag'

export const STATUS_FOR_FEATURE = gql`
  query featureStatus($envID: ID!, $feature: String!) {
    featureStatus(envID: $envID, feature: $feature) {
      environmentID
      feature
      version
      status
      configHash
      created
      lastModified
    }
  }
`

export const LOGS_FOR_FEATURE = gql`
  query featureLogs($envID: ID!, $feature: String!) {
    featureStatus(envID: $envID, feature: $feature) {
      log
    }
  }
`
