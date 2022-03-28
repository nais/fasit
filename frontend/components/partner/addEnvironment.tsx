import {Box, Modal} from '@mui/material'
import * as React from 'react'
import {useState} from 'react'
import {Fieldset, TextField} from '@navikt/ds-react'
import ErrorMessage from '../lib/error'
import RightJustifiedSubmitButton from '../lib/submitButton'
import {useForm} from 'react-hook-form'
import {yupResolver} from '@hookform/resolvers/yup'
import {useEnvironmentCreateMutation} from '../../lib/schema/graphql'
import * as yup from 'yup'
import {useRouter} from "next/router";

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

const newEnvironmentValidation = yup.object().shape({
    name: yup.string().required('Miljø trenger et navn'),
    description: yup.string(),
})

interface AddEnvironmentProps {
    open: boolean,
    onClose: (value: boolean) => void,
    partnerName: string
    partnerID: string
}

const AddEnvironment = ({open, onClose, partnerName, partnerID}: AddEnvironmentProps) => {
    const {register, handleSubmit, formState, reset} =
        useForm({
            resolver: yupResolver(newEnvironmentValidation),
        })

    const {errors} = formState
    const [backendError, setBackendError] = useState()
    const router = useRouter()

    const onSubmit = async (requestData: any) => {
        requestData.partnerID = partnerID
        try {
            await environmentCreate({
                variables: requestData,
                awaitRefetchQueries: true,
                refetchQueries: ['environmentsGet'],
            })
        } catch (e: any) {
            console.log(e)
            setBackendError(e)
        }

    }

    const closeAndReset = (data?: any) => {
        reset()
        onClose(false)

        if (data && data.environmentCreate) {
            router.push(`/partner/${partnerID}/${data.environmentCreate.id}`)
        }
    }

    const [environmentCreate] = useEnvironmentCreateMutation({
        onCompleted: closeAndReset
    })

    return (
        <Modal open={open} onClose={closeAndReset}>
            <Box sx={style}>
                <form onSubmit={handleSubmit(onSubmit)}>
                    Legg til miljø for {partnerName}
                    <Fieldset legend="Legg til nytt miljø" errorPropagation={false}>
                        {backendError && <ErrorMessage error={backendError}/>}
                        <TextField
                            id="name"
                            label="Navn"
                            {...register('name')}
                            error={errors.name?.message}
                        />
                        <TextField
                            id="description"
                            label="Beskrivelse"
                            {...register('description')}
                            error={errors.description?.message}
                        />
                        <RightJustifiedSubmitButton onCancel={closeAndReset}/>
                    </Fieldset>
                </form>
            </Box>
        </Modal>
    )
}
export default AddEnvironment