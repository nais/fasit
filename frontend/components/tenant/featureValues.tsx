import * as React from 'react'
import {
    EnvironmentGetByNamesQuery,
    EnvironmentGetByNamesQueryResult,
    EnvironmentGetQuery
} from '../../lib/schema/graphql'
import {Table} from '@navikt/ds-react'


interface FeatureValuesProps {
    values: EnvironmentGetQuery['environment']['values']

}

const FeatureValues = ({values}: FeatureValuesProps) => {
    return (
        <>
            <Table size={'small'}>
                <Table.Header>
                    <Table.Row>
                        <Table.HeaderCell>Key</Table.HeaderCell>
                        <Table.HeaderCell>Value</Table.HeaderCell>
                    </Table.Row>
                </Table.Header>
                <Table.Body>
                    { values.map((e) => (
                                <Table.Row>
                                    <Table.DataCell>
                                        {e.key}
                                    </Table.DataCell>
                                    <Table.DataCell>
                                        {e.value}
                                    </Table.DataCell>
                                </Table.Row>
                            )
                        )
                    }
                </Table.Body>
            </Table>

        </>
    )
}
export default FeatureValues
