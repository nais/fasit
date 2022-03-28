import * as React from 'react'
import styled from 'styled-components'
import ConfigPage from './configPage'
import { useFeaturesQuery } from '../../lib/schema/graphql'
import LoaderSpinner from '../lib/spinner'
import ErrorMessage from '../lib/error'
import { Success } from '@navikt/ds-icons'
import { navGronn } from '../../styles/constants'


const FeatureContainer = styled.div`
  border: 1px solid silver;
  padding: 10px;
  flex-grow: 1;
  border-radius: 0 5px 5px 5px;
`
const EmptyFeature = styled.div`
  border-left: 1px solid silver;
`

const FeatureStatus = styled.div`
  border: 1px solid silver;
  border-radius: 5px;
  padding: 10px;
  background-color: #f5f5f5;
  font-size: 0.8em;
  margin-bottom: 10px;
`

interface FeatureProps {
  envID: string,
  featureName: string,
}


const Feature = ({ envID, featureName }: FeatureProps) => {
  const { data, error, loading } = useFeaturesQuery()
  if (!envID || !featureName) { return <EmptyFeature /> }

  return (
    <FeatureContainer>
      <FeatureStatus>
        {error && <ErrorMessage error={error} />}
        {loading || !data && <LoaderSpinner />}
        {data?.features.filter((f) => f.name === featureName).map((f) => {
          return <div key={f.name} style={{ display: 'flex', flexDirection: 'column' }}>
            <div>status: <Success style={{ color: navGronn }} /></div>
            {f.chart && <div>chart: {f.chart}</div>}
            {f.repo && <div>repo: {f.repo}</div>}
            {f.source && <div>source: {f.source}</div>}
            {f.version && <div>version: {f.version}</div>}
          </div>
        })}
      </FeatureStatus>
      {
        featureName && envID && <ConfigPage envID={envID} feature={featureName} />
      }
    </FeatureContainer>
  )
}
export default Feature