import {Box, Modal} from '@mui/material'
import * as React from 'react'
import {useState} from 'react'
import ErrorMessage from '../lib/error'
import RightJustifiedSubmitButton from '../lib/submitButton'
import {useFeatureStateSaveMutation} from '../../lib/schema/graphql'

const style = {
    position: 'absolute' as 'absolute',
    top: '50%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    width: 300,
    bgcolor: 'background.paper',
    border: '2px solid #000',
    boxShadow: 24,
    p: 4,
}


interface EnableFeatureProps {
    open: boolean,
    onClose: React.Dispatch<boolean>,
    feature: string,
    envID: string,
    enabled: boolean
}

const EnableFeature = ({open, onClose, feature, envID, enabled}: EnableFeatureProps) => {
    const [backendError, setBackendError] = useState()
    const [save] = useFeatureStateSaveMutation()

    const onSubmit = async (e:  React.FormEvent<HTMLFormElement>) => {
        e.preventDefault()
        try {
            await save({
                variables: {feature, enabled: !enabled, envID},
                awaitRefetchQueries: true,
                refetchQueries: ['environmentGet'],
                onCompleted: () => onClose(false),
                onError:(e) => console.log(e)
            })
        } catch (e: any) {
            console.log(e)
            setBackendError(e)
        }

    }

    return (
        <Modal open={open} onClose={() => onClose(false)}>
            <Box sx={style}>
                {backendError && <ErrorMessage error={backendError}/>}
                <form onSubmit={onSubmit}>
                    Are you sure you want to {!enabled ? 'disable' : 'enable'} {feature}?
                        <RightJustifiedSubmitButton onCancel={() => onClose(false)}/>
                </form>
            </Box>
        </Modal>
    )
}
export default EnableFeature