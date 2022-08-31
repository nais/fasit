import * as React from 'react'
import styled from 'styled-components'
import {
  EnvironmentGetQuery,
  useHelmValuesQuery,
} from '../../lib/schema/graphql'
import { Loader } from '@navikt/ds-react'

const LogPre = styled.pre`
  overflow: auto;
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 14px;
`

interface FeatureProps {
  env: EnvironmentGetQuery['environment']
  featureName: string
}

const Feature = ({ env, featureName }: FeatureProps) => {
  const { loading, error, data } = useHelmValuesQuery({
    variables: { envID: env.id, feature: featureName },
  })

  return (
    <>
      {loading && <Loader transparent />}
      {error && <LogPre>{error.message}</LogPre>}
      {data && <LogPre>{JSON.stringify(data.helmValues, null, 2)}</LogPre>}
    </>
  )
}
export default Feature
