import * as React from 'react'
import {useState} from 'react'
import {Box, Modal} from '@mui/material'
import {Button} from '@navikt/ds-react'
import ErrorMessage from './error'
import {useConfigurationDeleteMutation} from '../../lib/schema/graphql'
import styled from 'styled-components'
import {Config} from "./configRows";

const style = {
    position: 'absolute' as 'absolute',
    top: '50%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    width: 600,
    bgcolor: 'background.paper',
    border: '2px solid #000',
    boxShadow: 24,
    p: 4,
}

const StyledButtonRow = styled.div`
  display: flex;
  gap: 10px;
  margin-top: 10px;
  justify-content: right;
`

interface ConfigDeleteProps {
    conf: Config
    open: boolean
    resetState: () => void
}


const ConfigDelete = ({conf, open, resetState}: ConfigDeleteProps) => {
    const [deleteConfig] = useConfigurationDeleteMutation()
    const [backendError, setBackendError] = useState()
    const deleteAndRefetchConfig = (id?: string) => {
        if (id) {
            deleteConfig({
                variables: {id},
                refetchQueries: ['configGet', 'configuration'],
                awaitRefetchQueries: true
            }).then(() => {
                resetState()
            }).catch((e: any) => {
                setBackendError(e)
            })
        }
    }
    return (

        <Modal open={open} onClose={resetState}>
            <Box sx={style}>
                Er du sikker på at du vil fjerne {conf.key}?
                <StyledButtonRow>
                    <Button variant='danger' onClick={() => deleteAndRefetchConfig(conf.id)}>Ja</Button>
                    <Button onClick={resetState}>Nei</Button>
                </StyledButtonRow>
                {
                    backendError && (
                        <ErrorMessage error={backendError}/>
                    )
                }
            </Box>
        </Modal>
    )
}
export default ConfigDelete
