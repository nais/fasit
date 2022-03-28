import gql from 'graphql-tag'

export const CONFIG_FOR_ENV = gql`
  query configGet($feature: String!, $envID: ID!) {
    envConfig(feature: $feature, envID: $envID) {
      id
      description
      value
      type
      env
      feature
      key
    }
  }
`
export const CONFIGURATION = gql`
  query configuration($feature: String!, $envID: ID) {
    configuration(feature: $feature, envID: $envID) {
      id
      environmentID
      feature
      description
      key
      value
      secret
    }
  }
`
