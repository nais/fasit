import type { NextPage } from 'next'
import { usePartnersGetQuery } from '../lib/schema/graphql'
import LoaderSpinner from '../components/lib/spinner'
import ErrorMessage from '../components/lib/error'
import styled from 'styled-components'
import IconBox from '../components/lib/icons/iconBox'
import Link from 'next/link'
import FeatureLogo from '../components/lib/icons/featureLogo'
import GetLogo from '../components/lib/getLogo'
import { Add } from '@navikt/ds-icons'

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
const Home: NextPage = () => {
  const partnersQuery = usePartnersGetQuery()
  if (partnersQuery.error) {
    return <ErrorMessage error={partnersQuery.error} />
  }
  if (partnersQuery.loading || !partnersQuery.data?.partners) {
    return <LoaderSpinner />
  }

  const partners = partnersQuery.data.partners

    return (
      <StyledMain>
        {partners.length > 0 ?
          <Links>
            {partners.map((p) => (
              <Link href={`/partner/${p.id}`} key={p.name}>
                <a>
                  <CategoryCard>
                    <IconBox size={50}>{GetLogo(p.name)}</IconBox>
                    <CategoryCardTitle>{p.name}</CategoryCardTitle>
                  </CategoryCard>
                </a>
              </Link>))}
            <Link href={`/partner/new`}>
              <a>
                <CategoryCard>
                  <IconBox size={50}><Add width={'40px'} height={'40px'} color={"#222"}/></IconBox>
                  <CategoryCardTitle>New partner</CategoryCardTitle>
                </CategoryCard>
              </a>
            </Link>
          </Links> : <div><p>No partners, partner!</p><Links>
              <Link href={`/partner/new`}>
                <a>
                  <CategoryCard>
                    <IconBox size={50}><Add width={'40px'} height={'40px'} color={"#222"}/></IconBox>
                    <CategoryCardTitle>New partner</CategoryCardTitle>
                  </CategoryCard>
                </a>
              </Link>
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
      </StyledMain>)
}

export default Home
