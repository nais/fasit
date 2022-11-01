import gql from 'graphql-tag'

export const FEATUREDETAILS = gql`
  query FeatureDetails {
    features {
      dependsOn {
        anyOf
        allOf
      }
      name
      chart
      config
      repo
      source
      environmentKinds
      version
      rolloutSummaries {
        id
        status
        created
      }
      configoverrides {
        keys
        environment {
          name
          tenant {
            name
          }
        }
      }
    }
  }
`
