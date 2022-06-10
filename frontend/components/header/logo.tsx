import styled from 'styled-components'
import Link from 'next/link'
import FasitLogo from '../lib/icons/fasit'


const LogoBox = styled.div`
  cursor: pointer;
  margin-right: 12px;
  height: 60px;
  width: 60px;
`

export const Logo = () => (
    <LogoBox>
        <Link href="/">
            <a>
              <FasitLogo/>
            </a>
        </Link>
    </LogoBox>
)

export default Logo
