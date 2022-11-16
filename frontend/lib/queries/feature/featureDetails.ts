import gql from 'graphql-tag'

export const FEATUREDETAILS = gql`
  query FeatureDetails($name: String!) {
    feature(name: $name) {
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
      outdatedInfo {
        newVersion
        dependency
        dependencyName
      }
    }
  }
`
