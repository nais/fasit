import gql from 'graphql-tag'

export const CONFIGURATION = gql`
  query configuration($feature: String!, $envID: ID) {
    configuration(feature: $feature, envID: $envID) {
      configuration {
        id
        feature {
          name
        }
        description
        type
        key
        value
        displayName
        secret
        chartValue
      }
      mapping {
        key
        value
        displayName
      }
    }
  }
`
