import gql from 'graphql-tag'

export const STATUS_FOR_FEATURE = gql`
  subscription statusForFeature($envID: ID!, $feature: String!) {
    status(envID: $envID, feature: $feature) {
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
