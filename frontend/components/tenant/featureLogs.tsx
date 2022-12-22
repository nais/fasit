import { Loader } from '@navikt/ds-react'
import { useEffect } from 'react'
import { useFeatureLogsQuery } from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import FeatureLogsView from './featureLogView'

interface FeatureProps {
  envID: string
  featureName: string
}

const FeatureLogs = ({ envID, featureName }: FeatureProps) => {
  const { loading, error, data, refetch } = useFeatureLogsQuery({
    variables: { envID: envID, feature: featureName },
    fetchPolicy: 'no-cache',
    nextFetchPolicy: 'no-cache',
  })

  useEffect(() => {
    refetch()
  })

  return (
    <>
      {loading && <Loader transparent />}
      {error && <ErrorMessage error={error} />}
      {data && (
        <>
          <FeatureLogsView logs={data.featureStatus.log} />
        </>
      )}
    </>
  )
}
export default FeatureLogs
