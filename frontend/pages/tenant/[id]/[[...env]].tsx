import { useRouter } from 'next/router'
import * as React from 'react'
import { useState } from 'react'
import { useEnvironmentsGetQuery, useTenantGetQuery } from '../../../lib/schema/graphql'
import ErrorMessage from '../../../components/lib/error'
import LoaderSpinner from '../../../components/lib/spinner'
import AddEnvironment from '../../../components/tenant/addEnvironment'
import { GetServerSideProps } from 'next'
import { addApolloState, initializeApollo } from '../../../lib/apollo'
import { TENANT_GET } from '../../../lib/queries/tenant/tenantGet'
import Environment from '../../../components/tenant/environment'
import { Main, MenuItem, MenuItems, MenuSeparator, PageContainer, SideMenu } from '../../../components/lib/PageLayout'


const Tenant = () => {
  const router = useRouter()
  const tenantID = router.query.id as string
  const envID = router.query.env as string


  const { data, error, loading } = useTenantGetQuery({ variables: { id: tenantID } })
  const envQuery = useEnvironmentsGetQuery({ variables: { tenantID: tenantID } })
  const [open, setOpen] = useState(false)

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
        {envQuery.loading || !envQuery.data && <LoaderSpinner />}
        {envQuery.error && <ErrorMessage error={envQuery.error} />}
        <MenuItems>
          {envQuery.data?.environments?.map((e, i) => {
            return (
              <MenuItem onClick={() => router.push(`/tenant/${tenantID}/${e.id}`)} key={`${e.name}_${i}`} active={e.id == envID}>
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
  const { id } = context.query

  const apolloClient = initializeApollo()

  try {
    await apolloClient.query({
      query: TENANT_GET,
      variables: { id },
    })
  } catch (e) {
    console.log(e)
  }

  return addApolloState(apolloClient, {
    props: { id },
  })
}
export default Tenant
