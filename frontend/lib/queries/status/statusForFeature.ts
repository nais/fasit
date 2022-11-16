import gql from 'graphql-tag'

export const LOGS_FOR_FEATURE = gql`
  query featureLogs($envID: ID!, $feature: String!) {
    featureStatus(envID: $envID, feature: $feature) {
      log
    }
  }
`
