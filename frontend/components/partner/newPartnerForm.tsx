import { yupResolver } from '@hookform/resolvers/yup'
import { useForm } from 'react-hook-form'
import ErrorMessage from '../lib/error'
import { useRouter } from 'next/router'
import { usePartnerCreateMutation } from '../../lib/schema/graphql'
import { Fieldset, TextField } from '@navikt/ds-react'
import RightJustifiedSubmitButton from '../lib/submitButton'
import { useState } from 'react'
import * as yup from 'yup'

const newPartnerValidation = yup.object().shape({
    name: yup.string().required('Partner trenger et navn'),
    description: yup.string(),
})

export const NewPartnerForm = () => {
    const router = useRouter()
    const {register, handleSubmit, formState} =
        useForm({
            resolver: yupResolver(newPartnerValidation),
        })

    const {errors} = formState
    const [backendError, setBackendError] = useState()

    const onSubmit = async (requestData: any) => {
        try {
            await partnerCreate({
                variables: requestData,
            })
        } catch (e: any) {
            console.log(e)
            setBackendError(e)
        }
    }

    const [partnerCreate] = usePartnerCreateMutation({
        onCompleted: (data) => {
            router.push(`/partner/${data.partnerCreate.id}`)
        }
    })

    return (
      <div style={{marginTop: '30px'}}>
        <form onSubmit={handleSubmit(onSubmit)}>
            <Fieldset legend="Add new partner" errorPropagation={false}>
                {backendError && <ErrorMessage error={backendError}/>}
                <TextField
                    id="name"
                    label="Name"
                    {...register('name')}
                    error={errors.name?.message}
                />
                <TextField
                    id="description"
                    label="Description"
                    {...register('description')}
                    error={errors.description?.message}
                />
                <RightJustifiedSubmitButton onCancel={() => { router.push('/')}} />
            </Fieldset>
        </form>
      </div>   )
}