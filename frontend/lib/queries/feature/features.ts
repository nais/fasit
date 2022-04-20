import gql from 'graphql-tag'

export const FEATURES = gql`
  query Features {
    features {
      dependsOn
      name
      chart
      config
      repo
      source
      version
    }
  }
`
