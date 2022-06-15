import * as React from 'react'
import {useState} from 'react'
import {Box, Modal} from '@mui/material'
import {Button} from '@navikt/ds-react'
import ErrorMessage from './error'
import {useConfigurationDeleteMutation} from '../../lib/schema/graphql'
import styled from 'styled-components'
import {Config} from "./configRows";
import {RightJustifiedButtons} from "./rightJustifiedButtons";

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
                refetchQueries: ['configuration'],
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
                <p style={{padding: '0px 0px 30px 0px'}}>Are you sure you want to delete {conf.key}?</p>
                <RightJustifiedButtons>
                    <Button style={{marginTop: "10px"}} onClick={resetState}>Cancel</Button>
                    <Button style={{marginTop: "10px", marginLeft: "10px"}} variant='danger'
                            onClick={() => deleteAndRefetchConfig(conf.id)}>Yes</Button>
                </RightJustifiedButtons>
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
