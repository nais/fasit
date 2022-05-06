import gql from 'graphql-tag'

export const CONFIG_FOR_ENV = gql`
  query configGet($feature: String!, $envID: ID!) {
    envConfig(feature: $feature, envID: $envID) {
      id
      description
      value
      type
      feature {
        name
      }
      key
    }
  }
`
export const CONFIGURATION = gql`
  query configuration($feature: String!, $envID: ID) {
    configuration(feature: $feature, envID: $envID) {
      id
      feature {
        name
      }
      description
      key
      value
      secret
    }
  }
`
