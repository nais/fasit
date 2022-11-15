import * as React from 'react'
import { useState } from 'react'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import {
  ConfigType,
  FeatureDetailsQuery,
  useConfigurationQuery,
} from '../../lib/schema/graphql'
import { Table } from '@navikt/ds-react'
import ConfigAdd from '../lib/configAdd'
import ConfigEdit from '../lib/configEdit'
import ConfigDelete from '../lib/configDelete'
import ConfigRows, { Config, Configs } from '../lib/configRows'

interface ConfigProps {
  feature: FeatureDetailsQuery['feature']
}

const ConfigPage = ({ feature }: ConfigProps) => {
  const { data, error, loading } = useConfigurationQuery({
    variables: { feature: feature.name },
  })
  const [currentConfig, setCurrentConfig] = useState<Config | undefined>()
  const [showDelete, setShowDelete] = useState(false)
  const [showUpdate, setShowUpdate] = useState(false)
  const [showCreate, setShowCreate] = useState(false)

  const resetConfig = () => {
    setCurrentConfig(undefined)
    setShowDelete(false)
    setShowCreate(false)
    setShowUpdate(false)
  }
  if (loading || !data) {
    return <LoaderSpinner />
  }

  return (
    <>
      {error && <ErrorMessage error={error} />}
      <Table size={'small'}>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              align={'center'}
              style={{ width: '50px' }}
            ></Table.HeaderCell>
            <Table.HeaderCell
              align={'center'}
              style={{ width: '50px' }}
            ></Table.HeaderCell>
            <Table.HeaderCell>Key</Table.HeaderCell>
            <Table.HeaderCell>Value</Table.HeaderCell>
            <Table.HeaderCell style={{ width: '100px' }} align="center">
              Actions
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          <ConfigRows
            configs={data.configuration.configuration}
            setCurrentConfig={setCurrentConfig}
            setShowUpdate={setShowUpdate}
            setShowDelete={setShowDelete}
            setShowCreate={setShowCreate}
            featurePage={true}
          />
        </Table.Body>
      </Table>

      {currentConfig && (
        <>
          <ConfigAdd
            conf={currentConfig}
            featureName={feature.name}
            open={showCreate}
            showOpen={resetConfig}
          />
          <ConfigEdit
            conf={currentConfig}
            featureName={feature.name}
            open={showUpdate}
            showOpen={resetConfig}
          />
          <ConfigDelete
            conf={currentConfig}
            open={showDelete}
            resetState={resetConfig}
          />
        </>
      )}
    </>
  )
}
export default ConfigPage
