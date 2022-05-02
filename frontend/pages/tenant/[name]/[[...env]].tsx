import {useRouter} from 'next/router'
import * as React from 'react'
import {useEffect, useState} from 'react'
import {
  useEnvironmentsGetLazyQuery,
  useEnvironmentsGetQuery,
  useTenantGetByNameQuery
} from '../../../lib/schema/graphql'
import ErrorMessage from '../../../components/lib/error'
import LoaderSpinner from '../../../components/lib/spinner'
import AddEnvironment from '../../../components/tenant/addEnvironment'
import {GetServerSideProps} from 'next'
import {addApolloState, initializeApollo} from '../../../lib/apollo'
import {TENANT_GET_BY_NAME} from '../../../lib/queries/tenant/tenantGet'
import Environment from '../../../components/tenant/environment'
import {Main, MenuItem, MenuItems, MenuSeparator, PageContainer, SideMenu} from '../../../components/lib/PageLayout'


const Tenant = () => {
  const router = useRouter()
  const name = router.query.name as string
  const envID = router.query.env as string
  const [tenantID, setTenantID] = useState("")
  const [open, setOpen] = useState(false)

  const { data, error, loading } = useTenantGetByNameQuery({ variables: { slug: name } })
  const [getEnvs,  envQuery] =  useEnvironmentsGetLazyQuery({ variables: { tenantID }})
  React.useEffect(() => {
    if (data) {
      setTenantID(data.tenant.id);
    }
    if (tenantID) {
      getEnvs()
    }
  }, [data, tenantID]);

  if (error) {
    return <ErrorMessage error={error} />
  }
  if (loading || !data?.tenant) {
    return <LoaderSpinner />
  }
  const tenant = data.tenant

  return (
    <PageContainer>
      <SideMenu>
        {envQuery.loading && envQuery.called && <LoaderSpinner />}
        {envQuery.error && <ErrorMessage error={envQuery.error} />}
        <MenuItems>
          {envQuery.data?.environments?.map((e, i) => {
            return (
              <MenuItem onClick={() => router.push(`/tenant/${name}/${e.id}`)} key={`${e.name}_${i}`} active={e.id == envID}>
                <a>{e.name}</a>
              </MenuItem>
            )
          })}
          {envQuery.data && envQuery.data.environments?.length > 0 && <MenuSeparator />}
          <MenuItem onClick={() => setOpen(true)}><a>Nytt miljø</a></MenuItem>
        </MenuItems>
      </SideMenu>
      <Main>
        {envID && <Environment envID={envID[0]} tenantName={tenant.name} />}
      </Main>
      <AddEnvironment open={open} onClose={() => setOpen(false)} tenantName={tenant.name}
        tenantID={tenant.id} />
    </PageContainer>
  )

}
export const getServerSideProps: GetServerSideProps = async (context) => {
  const { name } = context.query

  const apolloClient = initializeApollo()

  try {
    await apolloClient.query({
      query: TENANT_GET_BY_NAME,
      variables: { name },
    })
  } catch (e) {
    console.log(e)
  }

  return addApolloState(apolloClient, {
    props: { name },
  })
}
export default Tenant
