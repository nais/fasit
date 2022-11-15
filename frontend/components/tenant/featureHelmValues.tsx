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
  envID: string
  feature: {
    name: string
    version: string
    repo: string
    chart: string
  }
}

const Feature = ({ envID, feature }: FeatureProps) => {
  const { loading, error, data } = useHelmValuesQuery({
    variables: { envID: envID, feature: feature.name },
  })

  const helmInstall = [
    `helm upgrade --install ${feature.name}`,
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
