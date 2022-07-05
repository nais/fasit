import * as React from 'react'
import {Dispatch, FormEvent, SetStateAction, useEffect, useState} from 'react'
import {Box, Modal} from '@mui/material'
import {ConfigType, FeaturesQuery, useConfigurationCreateMutation} from '../../lib/schema/graphql'
import {Button, Switch, TextField} from '@navikt/ds-react'
import {useForm} from 'react-hook-form'
import KeywordsInput from './StringArrayInput'
import {Config} from "./configRows";
import ErrorMessage from "./error";
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

interface ConfigAddProps {
    conf: Config
    envID?: string
    globalConfig?: Config
    open: boolean
    feature: FeaturesQuery['features'][0],
    showOpen: Dispatch<SetStateAction<boolean>>
}

const ConfigAdd = ({conf, envID, globalConfig, feature, open, showOpen}: ConfigAddProps) => {
    const [createConfig] = useConfigurationCreateMutation()
    const [backendError, setBackendError] = useState(undefined)
    const [val, setVal] = useState<any>(undefined)
    const [intVal, setIntVal] = useState<number>(0)
    const [description, setDescription] = useState('')
    const {watch, formState, setValue} = useForm(globalConfig?.value && {defaultValues: {values: globalConfig.value}})

    const {errors} = formState
    const values = watch('values')
    useEffect(() => {
        setVal(values)
    }, [values])
    useEffect(() => {
        conf.type === ConfigType.Int && setVal(intVal)
    }, [intVal])
    useEffect(() => globalConfig?.value && setVal(globalConfig.value), [])

    const onDelete = (value: string) => {
        setValue('values', values.filter((v: string) => v !== value))
    }

    const onAdd = (value: string) => {
        values ?
            setValue('values', [...values, value]) :
            setValue('values', [value])
    }


    const featureConfig = feature.config[conf.key]

    const inputType = (type: ConfigType) => {
        switch (type) {
            case ConfigType.String:
                return <TextField placeholder={'value'} value={val} label={''} onChange={(e) => setVal(e.target.value)}></TextField>
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
                return <TextField value={intVal.toString()} label={''}
                                  onChange={(e) => setIntVal(Number(e.target.value))}></TextField>

        }
    }

    interface Variables {
        key: string,
        description: string,
        feature: string,
        value: any,
        environmentID?: string
    }


    const submit = async (e: FormEvent<HTMLFormElement>) => {
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
                <h1>Feature: {conf.feature}</h1>
                <h3>Key: <span style={{fontFamily: "Courier New, monospace"}}>{conf.key}</span></h3>
                <p>{conf.description}</p>
                {backendError && <ErrorMessage error={backendError}/>}

                <form onSubmit={(e) => submit(e)}>
                    {featureConfig && inputType(featureConfig.type)}
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
export default ConfigAdd