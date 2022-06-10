import Logo from "./header/logo";
import {Logout} from "@navikt/ds-icons";
import styled from "styled-components";
import {navOransje} from "../styles/constants";
import React from "react";
import {useUserInfoQuery} from "../lib/schema/graphql";
import LoaderSpinner from "./lib/spinner";

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


const Container = styled.div`
  min-height: 100vh;
  display: flex;
  flex-direction: column;
`
type pageLayoutProps = {
      children: React.ReactNode;
}

const PageLayout = ({children}: pageLayoutProps) => {
    const {data, loading, error} = useUserInfoQuery()
    if (loading) {
        return <LoaderSpinner/>
    }
    const email = data?.userInfo?.email
    return <Container>
        <HeaderBar role='banner'>
            <HeaderContent>
                <Logo/>
                {email ? <UserBox>{email}<a href={"/?gcp-iap-mode=CLEAR_LOGIN_COOKIE"}><Logout
                    title={"Log out"}/> </a></UserBox> : "unauthenticated"}
            </HeaderContent>
        </HeaderBar>
        <Main>{children}</Main>
    </Container>
}

export default PageLayout;