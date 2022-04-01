import * as React from 'react'
import {Dispatch, SetStateAction, useEffect, useState} from 'react'
import {Box, Modal} from '@mui/material'
import {ConfigType, useConfigurationCreateMutation, useFeaturesQuery} from '../../lib/schema/graphql'
import ErrorMessage from './error'
import LoaderSpinner from './spinner'
import {Button, Switch, TextField} from '@navikt/ds-react'
import {useForm} from 'react-hook-form'
import KeywordsInput from './StringArrayInput'
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

interface ConfigAddProps {
    conf: Config
    envID?: string
    globalConfig?: Config
    open: boolean
    showOpen: Dispatch<SetStateAction<boolean>>
}


const ConfigAdd = ({conf, envID, globalConfig, open, showOpen}: ConfigAddProps) => {
    const [createConfig] = useConfigurationCreateMutation()
    const {data, error, loading} = useFeaturesQuery()
    const [backendError, setBackendError] = useState(undefined)
    const [val, setVal] = useState<any>(undefined)
    const [intVal, setIntVal] = useState<number>(0)
    const [description, setDescription] = useState('')
    const {watch, formState, setValue} = useForm(globalConfig?.value && {defaultValues: {values: globalConfig.value}})

    const {errors} = formState
    const values = watch('values')
    useEffect(() => { setVal(values) }, [values])
    useEffect(() => { conf.type === ConfigType.Int && setVal(intVal) }, [intVal])
    useEffect(() => globalConfig?.value && setVal(globalConfig.value), [])

    const onDelete = (value: string) => {
        setValue('values', values.filter((v: string) => v !== value))
    }

    const onAdd = (value: string) => {
        values ?
            setValue('values', [...values, value]) :
            setValue('values', [value])
    }


    if (error) {
        <ErrorMessage error={error}/>
    }
    if (!data || loading) {
        <LoaderSpinner/>
    }
    const featureConfig = data?.features.filter((f) => {
        return f.name == conf.feature
    })[0].config[conf.key]

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
                return <Switch size='medium' position='left' checked={val} onChange={() => setVal(!val)}>
                    enable
                </Switch>
            case ConfigType.Int:
                return <TextField value={intVal} label={''} onChange={(e) => setIntVal(Number(e.target.value))}></TextField>

        }
    }

    interface Variables {
        key: string,
        description: string,
        feature: string,
        value: any,
        environmentID?: string
    }



    const submit = async (e: any, type: string) => {
        e.preventDefault()

        const variables: Variables = {
                key: conf.key,
                description: description,
                feature: conf.feature,
                value: val,
        }
        if (conf.type === ConfigType.Int) {
            variables.value = Number(val)
        }
        if (envID) {
            variables.environmentID = envID
        }

        try {
            await createConfig({
                variables,
                awaitRefetchQueries: true,
                refetchQueries: ['configGet', 'configuration'],
                onCompleted: () => {
                    showOpen(false)
                },
                onError: (e) => {
                    console.log(e)
                }
            })
        } catch (e: any) {
            setBackendError(e)
        }
    }
    return (
        <Modal open={open} onClose={() => showOpen(false)}>
            <Box sx={style}>
                <h1>{conf.feature}</h1>
                <h3>{conf.key} - {conf.type}</h3>

                <form onSubmit={(e) => submit(e, featureConfig.type)}>
                    {featureConfig && inputType(featureConfig.type)}
                    <TextField label={'Comment'} value={description} onChange={(e) => setDescription(e.target.value)}/>
                    <Button style={{marginTop: "10px"}}>Submit</Button>
                </form>
            </Box>
        </Modal>
    )
}
export default ConfigAdd
