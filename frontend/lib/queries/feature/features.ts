import gql from 'graphql-tag'

export const FEATURES = gql`
  query Features($kind: EnvironmentKind) {
    features(kind: $kind) {
      dependsOn
      name
      chart
      config
      repo
      source
      environmentKinds
      version
    }
  }
`
