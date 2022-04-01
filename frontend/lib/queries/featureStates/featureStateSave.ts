import gql from 'graphql-tag'

export const FEATURE_STATE_SAVE = gql`
  mutation featureStateSave(
    $envID: ID!
    $feature: String!
    $enabled: Boolean!
  ) {
    featureStateSave(envID: $envID, feature: $feature, enabled: $enabled) {
      enabled
    }
  }
`
