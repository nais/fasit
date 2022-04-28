import { yupResolver } from '@hookform/resolvers/yup'
import { useForm } from 'react-hook-form'
import ErrorMessage from '../lib/error'
import { useRouter } from 'next/router'
import { useTenantCreateMutation } from '../../lib/schema/graphql'
import { Fieldset, TextField } from '@navikt/ds-react'
import RightJustifiedSubmitButton from '../lib/submitButton'
import { useState } from 'react'
import * as yup from 'yup'

const newTenantValidation = yup.object().shape({
    name: yup.string().required('Tenant needs a name.'),
    description: yup.string(),
})

export const NewTenantForm = () => {
    const router = useRouter()
    const {register, handleSubmit, formState} =
        useForm({
            resolver: yupResolver(newTenantValidation),
        })

    const {errors} = formState
    const [backendError, setBackendError] = useState()

    const onSubmit = async (requestData: any) => {
        try {
            await tenantCreate({
                variables: requestData,
            })
        } catch (e: any) {
            console.log(e)
            setBackendError(e)
        }
    }

    const [tenantCreate] = useTenantCreateMutation({
        onCompleted: (data) => {
            router.push(`/tenant/${data.tenantCreate.id}`)
        }
    })

    return (
      <div style={{marginTop: '30px'}}>
        <form onSubmit={handleSubmit(onSubmit)}>
            <Fieldset legend="Add new tenant" errorPropagation={false}>
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