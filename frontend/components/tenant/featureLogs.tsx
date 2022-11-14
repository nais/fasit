import { Loader } from '@navikt/ds-react'
import {
  EnvironmentGetQuery,
  useFeatureLogsQuery,
} from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import FeatureLogsView from './featureLogView'

interface FeatureProps {
  env: EnvironmentGetQuery['environment']
  featureName: string
}

const Feature = ({ env, featureName }: FeatureProps) => {
  const { loading, error, data } = useFeatureLogsQuery({
    variables: { envID: env.id, feature: featureName },
  })

  return (
    <>
      {loading && <Loader transparent />}
      {error && <ErrorMessage error={error} />}
      {data && <FeatureLogsView logs={data.featureStatus.log} />}
    </>
  )
}
export default Feature
