import * as React from 'react'
import {useEffect, useState} from 'react'
import {
    EnvironmentKind,
    useEnvironmentGetByNamesQuery,
    useEnvironmentGetQuery,
    useEnvironmentUpdateMutation
} from '../../lib/schema/graphql'
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
import ManagementLogo from "../lib/icons/managementLogo";
import BreadCrumb from "../lib/breadcrumb";


const InfoBox = styled.div`
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

const TenantHeaderName = styled.h1`
  text-transform: capitalize;
  color: #222;
  margin: 0px;
`

const TenantHeader = styled.div`
  display: flex;
  flex-grow: 1;
  justify-content: space-between;
  text-transform: capitalize;
`
const TenantHeaderEnv = styled.h2`
  color: #696969;
  padding: 0px;
  margin: 0px;
`

const ManagementIcon = styled.div`
  margin: 0px 0px 0px 20px;
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

const Environment = ({ environmentName, tenantName }: EnvironmentProps) => {
  const [edit, setEdit] = useState(false)
  const [backendError, setBackendError] = useState()
  const [description, setDescription] = useState('')
  const [envUpdate] = useEnvironmentUpdateMutation()
  const { data, error, loading } = useEnvironmentGetByNamesQuery({ variables: { environmentName: environmentName[0], tenantName } })
  useEffect(() => { data?.environmentByNames?.description && setDescription(data.environmentByNames.description)}, [data])
  const router = useRouter()

  if (error) return <ErrorMessage error={error} />
  if (!data || loading) return <LoaderSpinner />
  const envID = data.environmentByNames.id


  const submit = () => {
    envUpdate(
      {
        variables: { description: description, id: envID },
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
      <ErrorMessage error={backendError} />
    )
  }

  const feature = router.query.feature as string
  const env = data.environmentByNames
  return (
    <div>
        <BreadCrumb />
        <TenantHeaderName>{tenantName}</TenantHeaderName><TenantHeader><TenantHeaderEnv>{`> ${env.name}`}</TenantHeaderEnv>{env.kind === EnvironmentKind.Management && <ManagementIcon><ManagementLogo /></ManagementIcon>}</TenantHeader>
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
          {edit ? <Close onClick={() => setEdit(false)} /> :
            <Edit onClick={() => setEdit(true)} />}
        </Icon>
        {edit && <SaveIcon onClick={() => submit()}><SaveFile /></SaveIcon>}
        </IconBox>
      </InfoBox>
      <TimeStamps>
        <span>Opprettet {humanizeDate(env.created)}</span>
        <span>Sist oppdatert {humanizeDate(env.lastModified)}</span>
      </TimeStamps>
      <Main>
        <FeaturesMenu env={env}/>
        <Feature env={env}  featureName={feature} />
      </Main>

    </div>
  )
}
export default Environment