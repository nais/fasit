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

const Code = styled.pre`
  border-radius: 5px;
  margin-top: 20px;
  font-size: 14px;
  background-color: #334;
  padding: 10px;
  color: white;

  &::selection {
    background: #75834f;
  }
`

interface FeatureProps {
  env: EnvironmentGetQuery['environment']
  featureName: string
}

const Feature = ({ env, featureName }: FeatureProps) => {
  const { loading, error, data } = useHelmValuesQuery({
    variables: { envID: env.id, feature: featureName },
  })

  const feature = env.featureStates.find(
    (f) => f.feature.name === featureName,
  )!.feature

  const helmInstall = [
    `helm upgrade --install ${featureName}`,
    `--namespace nais-system`,
    `--create-namespace`,
    `--version=${feature.version}`,
    `-f values.json`,
  ]

  if (feature.repo) {
    helmInstall.push(`--repo ${feature.repo}`)
  }

  helmInstall.push(feature.chart)

  return (
    <>
      {loading && <Loader transparent />}
      {error && <LogPre>{error.message}</LogPre>}
      {data && (
        <div>
          <Code>{helmInstall.join(' \\\n\t')}</Code>
          <hr />
          <LogPre>{JSON.stringify(data.helmValues, null, 2)}</LogPre>
        </div>
      )}
    </>
  )
}
export default Feature
