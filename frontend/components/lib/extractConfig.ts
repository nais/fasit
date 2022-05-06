import {QueryResult} from "@apollo/client";
import {ConfigGetQuery, ConfigGetQueryVariables, FeaturesQuery, FeaturesQueryVariables} from "../../lib/schema/graphql";
import {Configs} from "./configRows";

const extractConfig = (
    features: QueryResult<FeaturesQuery, FeaturesQueryVariables>,
    configQuery: QueryResult<ConfigGetQuery, ConfigGetQueryVariables>,
    featureName: string) => {
    let configs: Configs = {}
    let featureObject: FeaturesQuery['features'][0] | undefined
    if (features.data && configQuery.data) {
        featureObject = features.data.features.find((f) => f.name === featureName)
        const confKeys = featureObject?.config
        configQuery.data?.envConfig.forEach((c) => {
            if (c.__typename === 'EnvConfiguration') {
                configs[c.key] = {
                    id: c.id,
                    feature: c.feature.name,
                    key: c.key,
                    type: c.type,
                    value: c.value,
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
                    secret: false,
                    required: false,
                    enabled: false,
                    env: false,
                }
            }
        })
        Object.keys(confKeys).forEach((k) => {
            if (!configs[k]) {
                configs[k] = {
                    value: null,
                    env: false,
                    feature: featureName,
                    key: k,
                    type: confKeys[k].type,
                    secret: confKeys[k].secret,
                    required: confKeys[k].required,
                    enabled: confKeys[k].enabled
                }
            } else {
                configs[k].type = confKeys[k].type
                configs[k].secret = confKeys[k].secret
                configs[k].required = confKeys[k].required
                    configs[k].enabled = confKeys[k].enabled
                }
            },
        )
    }
    return {configs, featureObject};
}
export default extractConfig