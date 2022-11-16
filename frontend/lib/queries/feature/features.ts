import gql from 'graphql-tag'

export const FEATURELIST = gql`
  query FeatureList {
    features {
      name
      outdatedInfo {
        newVersion
        dependency
        dependencyName
      }
    }
  }
`
