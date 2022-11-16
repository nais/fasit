import gql from 'graphql-tag'

export const FEATURE_STATE = gql`
  query FeatureState($feature: String!, $envID: ID!) {
    featureState(feature: $feature, envID: $envID) {
      enabled
      rolloutStatus
      missingDependencies {
        name
      }
      feature {
        name
        dependsOn {
          anyOf
          allOf
        }
        chart
        repo
        source
        version
      }
      configuration {
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
  }
`
