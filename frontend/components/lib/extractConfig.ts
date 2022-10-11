import { QueryResult } from "@apollo/client"
import {
  ConfigurationQuery,
  ConfigurationQueryVariables,
  FeaturesQuery,
  FeaturesQueryVariables,
} from "../../lib/schema/graphql"
import { Configs } from "./configRows"

const extractConfig = (
  features: QueryResult<FeaturesQuery, FeaturesQueryVariables>,
  configQuery: QueryResult<ConfigurationQuery, ConfigurationQueryVariables>,
  featureName: string) => {
  let configs: Configs = {}
  let featureObject: FeaturesQuery["features"][0] | undefined
  if (features.data && configQuery.data) {
    featureObject = features.data.features.find((f) => f.name === featureName)
    const confKeys = featureObject?.config
    configQuery.data?.configuration.configuration.forEach((c) => {
      if (c.__typename === "EnvConfiguration") {
        configs[c.key] = {
          id: c.id,
          feature: c.feature.name,
          key: c.key,
          type: c.type,
          value: c.value,
          displayName: c.displayName,
          description: c.description,
          secret: false,
          required: false,
          enabled: false,
          env: true,
        }
      } else {
        configs[c.key] = {
          id: c.id,
          feature: c.feature.name,
          key: c.key,
          type: c.type,
          value: c.value,
          description: c.description,
          displayName: c.displayName,
          secret: false,
          required: false,
          enabled: false,
          env: false,
        }
      }
    })
    Object.keys(confKeys).forEach((k) => {
        if (configs[k]) {
          configs[k].type = confKeys[k].type
          configs[k].secret = confKeys[k].secret
          configs[k].required = confKeys[k].required
          configs[k].enabled = confKeys[k].enabled
        }
      },
    )
  }
  return { configs, featureObject }
}
export default extractConfig