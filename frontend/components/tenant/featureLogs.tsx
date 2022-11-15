import { Loader } from '@navikt/ds-react'
import { useFeatureLogsQuery } from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import FeatureLogsView from './featureLogView'

interface FeatureProps {
  envID: string
  featureName: string
}

const Feature = ({ envID, featureName }: FeatureProps) => {
  const { loading, error, data } = useFeatureLogsQuery({
    variables: { envID: envID, feature: featureName },
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
