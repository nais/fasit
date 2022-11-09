import Link from 'next/link'
import { useRouter } from 'next/router'
import styled from 'styled-components'
import BreadCrumb from '../../../components/lib/breadcrumb'
import ErrorMessage from '../../../components/lib/error'
import {
  Main,
  MenuItem,
  MenuItems,
  PageContainer,
  SideMenu,
} from '../../../components/lib/PageLayout'
import LoaderSpinner from '../../../components/lib/spinner'
import Environment from '../../../components/tenant/environment'
import {
  TenantGetByNameQuery,
  useTenantGetByNameQuery,
} from '../../../lib/schema/graphql'
import {
  navMorkGra,
  navRod,
  redError,
  redErrorLighten80,
} from '../../../styles/constants'

const WarningLink = styled.a`
  color: ${navMorkGra};
  cursor: pointer;
  background-color: #fde8e6;
  border: 1px solid ${redError};
  border-radius: 5px;
  padding: 0.5em 1em;
  margin: 5px;
  :hover {
    background-color: ${redErrorLighten80};
  }
`

const WarningDot = styled.div`
  margin-left: -40px;
  margin-right: 20px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background-color: ${navRod};
  color: white;
  display: inline-flex;
  justify-content: center;
  align-items: center;
  font-size: 14px;
  font-weight: bold;
`

const warningLink = (
  tenant: TenantGetByNameQuery['tenant'],
  w: TenantGetByNameQuery['tenant']['warnings'][0],
  i: number,
) => {
  if (w.__typename === 'NaisdWarning') {
    return (
      <Link href={`/tenant/${tenant.name}/${w.environment.name}`} key={i}>
        <WarningLink>
          {`${w.message} in `}
          <strong>{w.environment.name}</strong>
        </WarningLink>
      </Link>
    )
  }
  if (w.__typename === 'FeatureWarning') {
    return (
      <Link
        href={`/tenant/${tenant.name}/${w.environment.name}?feature=${w.feature.name}`}
        key={i}
      >
        <WarningLink>
          <strong>{w.feature.name}</strong>
          {` in `}
          <strong>{w.environment.name}</strong>: {w.message}
        </WarningLink>
      </Link>
    )
  }
  return (
    <Link key={i} href="#">
      <WarningLink key={i}>{w.message}</WarningLink>
    </Link>
  )
}

const Tenant = () => {
  const router = useRouter()
  const tenantName = router.query.tenantName as string
  const environmentName = router.query.environmentName as string

  const { data, error, loading } = useTenantGetByNameQuery({
    variables: { slug: tenantName },
  })

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
          <span style={{ marginBottom: '15px' }}>Environments</span>

          {tenant.environments?.map((e, i) => {
            return (
              <MenuItem
                onClick={() => router.push(`/tenant/${tenantName}/${e.name}`)}
                key={`${e.name}_${i}`}
                active={e.name == environmentName}
                style={{ position: 'relative' }}
              >
                {e.warnings.length > 0 && (
                  <WarningDot>{e.warnings.length}</WarningDot>
                )}
                <a>{e.name}</a>
              </MenuItem>
            )
          })}
        </MenuItems>
      </SideMenu>
      <Main>
        <BreadCrumb />
        {environmentName ? (
          <Environment
            environmentName={environmentName}
            tenantName={tenant.name}
          />
        ) : (
          <>
            <div> Tenant status</div>
            {tenant.warnings.map((w, i) => warningLink(tenant, w, i))}
          </>
        )}
      </Main>
    </PageContainer>
  )
}

export default Tenant
