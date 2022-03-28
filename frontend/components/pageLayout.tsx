import styled from 'styled-components'
import Logo from './header/logo'

const HeaderBar = styled.header`
  > div {
    width: 80vw;
    display: flex;
    justify-content: space-between;
  }
  display: flex;
  justify-content: center;
  height: 60px;
  border-bottom: 1px solid #aaa;
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

export const PageLayout = ({ children }: { children: React.ReactNode }) => {

  return (
    <Container>
      <HeaderBar role='banner'>
        <div>
          <Logo />
        </div>
      </HeaderBar>
      <Main>{children}</Main>
    </Container>
  )
}

export default PageLayout
