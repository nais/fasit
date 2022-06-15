import * as React from 'react'
import {Dispatch, SetStateAction, useEffect, useState} from 'react'
import {Box, Modal} from '@mui/material'
import {ConfigType, useConfigurationUpdateMutation} from '../../lib/schema/graphql'
import ErrorMessage from './error'
import {Button, Switch, TextField} from '@navikt/ds-react'
import {useForm} from 'react-hook-form'
import KeywordsInput from './StringArrayInput'
import {ApolloError} from '@apollo/client'
import {Config} from "./configRows";
import styled from "@emotion/styled";
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

interface ConfigEditProps {
    conf: Config
    open: boolean
    showOpen: Dispatch<SetStateAction<boolean>>
}


const ConfigEdit = ({conf, open, showOpen}: ConfigEditProps) => {
    const [updateConfig] = useConfigurationUpdateMutation()
    const [backendError, setBackendError] = useState<ApolloError | undefined>(undefined)
    const [val, setVal] = useState<any>(undefined)
    const [description, setDescription] = useState('')
    const {watch, formState, setValue} = useForm({defaultValues: {values: conf.value}})

    const {errors} = formState
    const values = watch('values')

    useEffect(() => {
        setVal(conf.value)
    }, [])
    useEffect(() => {
        setVal(values)
    }, [values])

    const onDelete = (value: string) => {
        setValue('values', values.filter((v: string) => v !== value))
    }

    const onAdd = (value: string) => {
        values ?
            setValue('values', [...values, value]) :
            setValue('values', [value])
    }

    const resetAndClose = () => {
        showOpen(false)
    }

    const inputType = (type: ConfigType) => {
        switch (type) {
            case ConfigType.String:
                return <TextField value={val} label={''} onChange={(e) => setVal(e.target.value)}></TextField>
            case ConfigType.StringArray:
                return <KeywordsInput
                    onAdd={onAdd}
                    onDelete={onDelete}
                    values={values || []}
                    error={errors.values?.[0].message}
                />
            case ConfigType.Bool:
                return <Switch size='medium' position='left' checked={val}
                               onChange={() => setVal(!val)}> enable </Switch>
            case ConfigType.Int:
                return <TextField value={val} label={''} onChange={(e) => setVal(e.target.value)}></TextField>
            default:
                console.log("unknown type", type)

        }
    }

    interface Variables {
        id: string,
        description: string,
        value: any,
    }

    const submit = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault()
        if (!conf.id) {
            setBackendError(new ApolloError({errorMessage: "Attempting to create global configuration in an environment scope"}))
            return
        }

        const variables: Variables = {
            id: conf.id,
            description: description,
            value: val
        }
        if (conf.type === ConfigType.Int) {
            variables.value = Number(val)
        }
        try {
            await updateConfig({
                variables: variables,
                awaitRefetchQueries: true,
                refetchQueries: ['configGet', 'configuration'],
                onCompleted: () => {
                    showOpen(false)
                },
                onError: (e) => {
                    setBackendError(e)
                    console.log(e)
                }
            })
        } catch (e: any) {
            setBackendError(e)
        }
    }


    return (
        <Modal open={open} onClose={resetAndClose}>
            <Box sx={style}>
                <h1>{conf.feature}</h1>
                <h3>{conf.key}</h3>
                {backendError && <ErrorMessage error={backendError}/>}
                <form onSubmit={(e) => submit(e)}>
                    {inputType(conf.type)}
                    <br/>
                    <TextField style={{marginTop: '20px'}} label={'Comment'} value={description} onChange={(e) => setDescription(e.target.value)}/>
                    <RightJustifiedButtons>
                        <Button variant={"danger"} style={{marginTop: "10px"}}
                                onClick={() => showOpen(false)}>Cancel</Button>
                        <Button style={{marginTop: "10px", marginLeft: "10px"}}>Submit</Button>
                    </RightJustifiedButtons>
                </form>
            </Box>
        </Modal>
    )
}
export default ConfigEdit
