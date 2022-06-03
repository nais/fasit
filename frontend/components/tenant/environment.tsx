import * as React from 'react'
import {useEffect, useState} from 'react'
import {useEnvironmentGetByNamesQuery, useEnvironmentUpdateMutation} from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import humanizeDate from '../lib/humanizeDate'
import styled from 'styled-components'
import {Close, Edit, SaveFile} from '@navikt/ds-icons'
import FeaturesMenu from './featuresMenu'
import Feature from './feature'
import {useRouter} from 'next/router'
import {navGronn, navRod} from '../../styles/constants'
import {Textarea} from '@navikt/ds-react'
import EnvironmentStatusPage from "./environmentStatusPage";
import FeatureValues from "./featureValues";


const InfoBox = styled.div`
  margin-top: 25px;
  display: flex;
  border: 1px solid silver;
  border-radius: 5px;
  background-color: #f5f5f5;
  padding: 10px 10px 0 10px;
  font-size: 0.8em;
`
const Description = styled.pre`
  font-family: var(--navds-font-family);
  font-size: 1em;
  margin-top: 0px;
  margin-bottom: 0px;
`

const SaveIcon = styled.div`
   svg {
     :hover {
       color: ${navGronn};
       cursor: pointer;
     }
   }
`
const Icon = styled.div`
   svg {
     :hover {
       color: ${navRod};
       cursor: pointer;
     }
   }
`
const IconBox = styled.div`
  display:flex;
  flex-direction: column;
  justify-content: space-between;
`
const DescriptionBox = styled.div`
  flex-grow: 1;
  padding-bottom: 25px;
`

const TimeStamps = styled.div`
  display: flex;
  flex-direction: column;
  font-size: 0.6em;
  flex-grow: 1;
  text-align: right;
`
const Main = styled.div`
  margin-top: 10px;
  display: flex;
`

interface EnvironmentProps {
    environmentName: string,
    tenantName: string;
}

const Environment = ({environmentName, tenantName}: EnvironmentProps) => {
    const [edit, setEdit] = useState(false)
    const [backendError, setBackendError] = useState()
    const [description, setDescription] = useState('')
    const [envUpdate] = useEnvironmentUpdateMutation()
    const {data, error, loading} = useEnvironmentGetByNamesQuery({
        variables: {
            environmentName: environmentName[0],
            tenantName
        }
    })
    useEffect(() => {
        data?.environmentByNames?.description && setDescription(data.environmentByNames.description)
    }, [data])
    const router = useRouter()

    if (error) return <ErrorMessage error={error}/>
    if (!data || loading) return <LoaderSpinner/>
    const feature = router.query.feature as string
    const env = data.environmentByNames

    const submit = () => {
        envUpdate(
            {
                variables: {description: description, id: env.id},
                awaitRefetchQueries: true,
                refetchQueries: ['environmentGet'],
            }).then(() => {
            setBackendError(undefined)
            setDescription("")
            setEdit(false)
        }).catch((e: any) => {
            setBackendError(e)
        })
    }
    {
        backendError && (
            <ErrorMessage error={backendError}/>
        )
    }

    return (
        <div>
            <InfoBox>
                <DescriptionBox>
                    {edit ?
                        <Textarea
                            label={'description'}
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                        /> :
                        <Description>
                            {env.description}
                        </Description>
                    }
                </DescriptionBox>
                <IconBox>

                    <Icon>
                        {edit ? <Close onClick={() => setEdit(false)}/> :
                            <Edit onClick={() => setEdit(true)}/>}
                    </Icon>
                    {edit && <SaveIcon onClick={() => submit()}><SaveFile/></SaveIcon>}
                </IconBox>
            </InfoBox>
            <TimeStamps>
                <span>Opprettet {humanizeDate(env.created)}</span>
                <span>Sist oppdatert {humanizeDate(env.lastModified)}</span>
            </TimeStamps>
            <Main>
                <FeaturesMenu env={env}/>
                {feature ?
                    <Feature env={env} featureName={feature}/> :
                    <EnvironmentStatusPage env={env}/>
                }
            </Main>

        </div>
    )
}
export default Environment