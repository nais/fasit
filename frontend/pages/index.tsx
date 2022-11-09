import type { NextPage } from 'next'
import { useTenantsGetQuery } from '../lib/schema/graphql'
import LoaderSpinner from '../components/lib/spinner'
import ErrorMessage from '../components/lib/error'
import styled from 'styled-components'
import IconBox from '../components/lib/icons/iconBox'
import Link from 'next/link'
import FeatureLogo from '../components/lib/icons/featureLogo'
import GetLogo from '../components/lib/getLogo'
import { navOransje, navRod } from '../styles/constants'

const Links = styled.div`
  display: flex;
  justify-content: center;
  flex-direction: row;
  margin-top: 100px;
  gap: 70px;
  flex-wrap: wrap;
  position: relative;
  a {
    text-decoration: none;
  }
`
const CategoryCard = styled.div`
  display: flex;
  position: relative;
  flex-direction: column;
  justify-content: center;
  border: 1px solid rgb(240, 240, 240);
  border-radius: 5px;
  box-shadow: rgb(239, 239, 239) 0px 0px 30px 0px;
  width: 150px;
  height: 150px;
  padding: 20px;
  :hover {
    box-shadow: rgb(239, 239, 239) 0px 1px 0px 0.5px;
  }
`
const CategoryCardTitle = styled.div`
  display: flex;
  justify-content: center;
  padding-top: 5px;
  color: #222;
`

const StyledMain = styled.div`
  display: flex;
  justify-content: center;
`

const WarningDot = styled.div`
  position: absolute;
  top: -5px;
  right: -5px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background-color: ${navRod};
  color: white;
  display: flex;
  justify-content: center;
  align-items: center;
  font-size: 14px;
  font-weight: bold;
`

const WarningDotOrange = styled(WarningDot)`
  background-color: ${navOransje};
`

const Home: NextPage = ({ email }: any) => {
  const tenantsGetQuery = useTenantsGetQuery()
  if (tenantsGetQuery.error) {
    return <ErrorMessage error={tenantsGetQuery.error} />
  }
  if (tenantsGetQuery.loading || !tenantsGetQuery.data?.tenants) {
    return <LoaderSpinner />
  }

  const tenants = tenantsGetQuery.data.tenants

  return (
    <StyledMain>
      {tenants.length > 0 ? (
        <Links>
          {tenants.map((p) => (
            <Link href={`/tenant/${p.name}`} key={p.name}>
              <a>
                <CategoryCard>
                  {p.warnings.length > 0 && (
                    <WarningDot>{p.warnings.length}</WarningDot>
                  )}
                  <IconBox size={50}>{GetLogo(p.name)}</IconBox>
                  <CategoryCardTitle>{p.name}</CategoryCardTitle>
                </CategoryCard>
              </a>
            </Link>
          ))}
          <Link href={'/features/'}>
            <a>
              <CategoryCard>
                {tenantsGetQuery.data.outdatedInfo.length > 0 && (
                  <WarningDotOrange />
                )}
                <IconBox size={50}>
                  <FeatureLogo />
                </IconBox>
                <CategoryCardTitle>features</CategoryCardTitle>
              </CategoryCard>
            </a>
          </Link>
        </Links>
      ) : (
        <div>
          <p>No tenants, compadre!</p>
          <Links>
            <Link href={'/features/'}>
              <a>
                <CategoryCard>
                  <IconBox size={50}>
                    <FeatureLogo />
                  </IconBox>
                  <CategoryCardTitle>features</CategoryCardTitle>
                </CategoryCard>
              </a>
            </Link>
          </Links>
        </div>
      )}
    </StyledMain>
  )
}

export default Home
