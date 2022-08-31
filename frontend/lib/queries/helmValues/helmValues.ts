import gql from 'graphql-tag'

export const HELM_VALUES = gql`
  query helmValues($feature: String!, $envID: ID!) {
    helmValues(feature: $feature, envID: $envID)
  }
`
