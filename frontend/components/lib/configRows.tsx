import {Table} from "@navikt/ds-react";
import {ConfigType} from "../../lib/schema/graphql";
import prettifyArray from "./prettifyArray";
import {Add, Delete, FileContent, Globe, Place, Success, Warning, Wrench} from "@navikt/ds-icons";
import {navGronn, navRod} from "../../styles/constants";
import * as React from "react";
import styled from "styled-components";

const Center = styled.div`
  display: flex;
  gap: 10px;
  justify-content: center;
`

const StyledWrench = styled(Wrench)`
  :hover{
    color: ${navGronn};
  }
  cursor: pointer;
`

const StyledDelete = styled(Delete)`
  :hover{
    color: ${navRod};
  }
  cursor: pointer;
`

const StyledAdd = styled(Add)`
  :hover{
    color: ${navGronn};
  }
  cursor: pointer;
`

export interface Config {
    id?: string
    description?: string | null
    value: any
    type: ConfigType
    env: boolean
    feature: string
    key: string
    secret: boolean
    required: boolean
    enabled?: boolean
}

export interface Configs {
    [index: string]: Config
}

interface ConfigRowProps {
    configs: Configs,
    keys: string[],
    setCurrentConfig: React.Dispatch<Config>,
    setShowUpdate: React.Dispatch<boolean>
    setShowDelete: React.Dispatch<boolean>
    setShowCreate: React.Dispatch<boolean>
    featurePage?: boolean
}

const ConfigRows = ({
                        configs,
                        keys,
                        setCurrentConfig,
                        setShowUpdate,
                        setShowCreate,
                        setShowDelete,
                        featurePage
                    }: ConfigRowProps) => {

    return <>{keys.map((key) => {
            const conf = configs[key]
            return (
                <Table.Row key={key}>
                    <Table.DataCell>{key}</Table.DataCell>
                    <Table.DataCell>{conf.secret ? '*****' : conf.type != ConfigType.StringArray ?
                        conf.value != null ? JSON.stringify(conf.value).replace(/"/g, '') :
                            '<default>' :
                        prettifyArray(conf.value)}
                    </Table.DataCell>
                    <Table.DataCell align={'center'}>{conf.env ? <Place/> : JSON.stringify(conf.value) !== "null" ? <Globe/> :
                        <FileContent/>}
                    </Table.DataCell>
                    <Table.DataCell align={'center'}>{conf.required &&
                        <Center>
                            {conf.value ? <Success style={{color: navGronn}}/> :
                            <Warning style={{color: navRod}}/>}
                        </Center>}
                    </Table.DataCell>
                    <Table.DataCell align={'center'}>
                        <Center> {conf.env || (featurePage && conf.value != null) ?
                            <>
                                <StyledWrench onClick={() => {
                                    setCurrentConfig(conf)
                                    setShowUpdate(true)
                                }}
                                />
                                <StyledDelete onClick={() => {
                                    setCurrentConfig(conf)
                                    setShowDelete(true)
                                }}
                                />
                            </> :
                            <StyledAdd onClick={() => {
                                setCurrentConfig(conf)
                                setShowCreate(true)
                            }}/>}
                        </Center>
                    </Table.DataCell>
                    <Table.DataCell>{conf.description}</Table.DataCell>
                </Table.Row>
            )
        }
    )}</>
}
export default ConfigRows
