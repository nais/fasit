import {useRouter} from 'next/router'
import * as React from 'react'
import {useState} from 'react'
import {useEnvironmentsGetLazyQuery, useTenantGetByNameQuery} from '../../../lib/schema/graphql'
import ErrorMessage from '../../../components/lib/error'
import LoaderSpinner from '../../../components/lib/spinner'
import AddEnvironment from '../../../components/tenant/addEnvironment'
import Environment from '../../../components/tenant/environment'
import {Main, MenuItem, MenuItems, MenuSeparator, PageContainer, SideMenu} from '../../../components/lib/PageLayout'


const Tenant = () => {
  const router = useRouter()
  const tenantName = router.query.tenantName as string
  const environmentName = router.query.environmentName as string
  const [tenantID, setTenantID] = useState("")
  const [open, setOpen] = useState(false)

  const { data, error, loading } = useTenantGetByNameQuery({ variables: { slug: tenantName } })
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
              <MenuItem onClick={() => router.push(`/tenant/${tenantName}/${e.name}`)} key={`${e.name}_${i}`} active={e.name == environmentName}>
                <a>{e.name}</a>
              </MenuItem>
            )
          })}
          {envQuery.data && envQuery.data.environments?.length > 0 && <MenuSeparator />}
          <MenuItem onClick={() => setOpen(true)}><a>Nytt miljø</a></MenuItem>
        </MenuItems>
      </SideMenu>
      <Main>
        {environmentName && <Environment environmentName={environmentName} tenantName={tenant.name} />}
      </Main>
      <AddEnvironment open={open} onClose={() => setOpen(false)} tenantName={tenant.name}
        tenantID={tenant.id} />
    </PageContainer>
  )
}

export default Tenant
