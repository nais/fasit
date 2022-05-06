import {useRouter} from 'next/router'
import * as React from 'react'
import {useTenantGetByNameQuery} from '../../../lib/schema/graphql'
import ErrorMessage from '../../../components/lib/error'
import LoaderSpinner from '../../../components/lib/spinner'
import Environment from '../../../components/tenant/environment'
import {Main, MenuItem, MenuItems, MenuSeparator, PageContainer, SideMenu} from '../../../components/lib/PageLayout'
import BreadCrumb from "../../../components/lib/breadcrumb";


const Tenant = () => {
  const router = useRouter()
  const tenantName = router.query.tenantName as string
  const environmentName = router.query.environmentName as string

  const { data, error, loading } = useTenantGetByNameQuery({ variables: { slug: tenantName } })

  if (error) {
    return <ErrorMessage error={error} />
  }
  if (loading || !data) {
    return <LoaderSpinner />
  }
  const tenant = data.tenant

  return (
    <PageContainer>
      <SideMenu>
        <MenuItems>
          {tenant.environments?.map((e, i) => {
            return (
              <MenuItem onClick={() => router.push(`/tenant/${tenantName}/${e.name}`)} key={`${e.name}_${i}`} active={e.name == environmentName}>
                <a>{e.name}</a>
              </MenuItem>
            )
          })}
        </MenuItems>
      </SideMenu>
      <Main>
        <BreadCrumb/>
        {environmentName ?
            <Environment environmentName={environmentName} tenantName={tenant.name} /> :
            <div> Tenant status</div>
        }
      </Main>
    </PageContainer>
  )
}

export default Tenant
