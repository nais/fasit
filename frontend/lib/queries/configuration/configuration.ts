import gql from 'graphql-tag'

export const CONFIGURATION = gql`
  query configuration($feature: String!, $envID: ID) {
    configuration(feature: $feature, envID: $envID) {
      configuration {
        id
        description
        type
        key
        value
        displayName
        secret
        chartValue
        required
      }
      mapping {
        key
        value
        displayName
      }
    }
  }
`
