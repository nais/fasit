import type { NextPage } from 'next'
import { useTenantsGetQuery } from '../lib/schema/graphql'
import LoaderSpinner from '../components/lib/spinner'
import ErrorMessage from '../components/lib/error'
import styled from 'styled-components'
import IconBox from '../components/lib/icons/iconBox'
import Link from 'next/link'
import FeatureLogo from '../components/lib/icons/featureLogo'
import GetLogo from '../components/lib/getLogo'
import Logo from "../components/header/logo";
import {Logout} from "@navikt/ds-icons";
import {navOransje} from "../styles/constants";

const HeaderBar = styled.header`
  width: 100%;
  border-bottom: 1px solid #aaa;
  justify-content: center;
  height: 60px;
`
const HeaderContent = styled.div`
  width: 80vw;
  display: flex;
  margin: 0 auto;
  justify-content: space-between;
  align-items: center;
  height: 60px;
`
const UserBox = styled.span`
    display: flex;
    align-items: center;
    color: #333;
    justify-content: center;
    > a {
        color: #333;
        margin-left: 10px;
        margin-top: 5px;
        cursor: pointer;
        :hover {
          color: ${navOransje};
        }
    }
`

const Container = styled.div`
  min-height: 100vh;
  display: flex;
  flex-direction: column;
`
const Main = styled.main`
  width: 80vw;
  
  @media only screen and (max-width: 768px) {
    width: 95vw;
  }
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  flex-grow: 1;
`
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
    flex-direction: column;
    justify-content: center;
    border: 1px solid rgb(240, 240, 240);
    border-radius: 5px;
    box-shadow: rgb(239, 239, 239) 0px 0px 30px 0px;
    width: 150px;
    height: 150px;
    padding: 20px;
    :hover{
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
  justify-content: center

`
const Home: NextPage = ({email}: any) => {
  const tenantsGetQuery = useTenantsGetQuery()
  if (tenantsGetQuery.error) {
    return <ErrorMessage error={tenantsGetQuery.error} />
  }
  if (tenantsGetQuery.loading || !tenantsGetQuery.data?.tenants) {
    return <LoaderSpinner />
  }

  const tenants = tenantsGetQuery.data.tenants

    return (
        <Container>
          <HeaderBar role='banner'>
              <HeaderContent>
                  <Logo />
                  {email ? <UserBox > {email} <a href={"/?gcp-iap-mode=CLEAR_LOGIN_COOKIE"}><Logout title={"Log out"}/> </a></UserBox>: "unauthenticated"}
              </HeaderContent>
          </HeaderBar>
          <Main>
            <StyledMain>
              {tenants.length > 0 ?
                  <Links>
                    {tenants.map((p) => (
                        <Link href={`/tenant/${p.name}`} key={p.name}>
                          <a>
                            <CategoryCard>
                              <IconBox size={50}>{GetLogo(p.name)}</IconBox>
                              <CategoryCardTitle>{p.name}</CategoryCardTitle>
                            </CategoryCard>
                          </a>
                        </Link>))}
                    <Link href={"/features/"}>
                      <a>
                        <CategoryCard>
                          <IconBox size={50}><FeatureLogo/></IconBox>
                          <CategoryCardTitle>features</CategoryCardTitle>
                        </CategoryCard>
                      </a>
                    </Link>
                  </Links> : <div><p>No tenants, compadre!</p><Links>
                    <Link href={"/features/"}>
                      <a>
                        <CategoryCard>
                          <IconBox size={50}><FeatureLogo/></IconBox>
                          <CategoryCardTitle>features</CategoryCardTitle>
                        </CategoryCard>
                      </a>
                    </Link>
                  </Links></div>
              }
            </StyledMain>
          </Main>
        </Container>
    )
}
export async function getServerSideProps({req} :any) {
  return {
    props: {email: req.headers["x-goog-authenticated-user-email"] || null}, // will be passed to the page component as props
  }
}


export default Home
