import styled from 'styled-components'
import Link from 'next/link'
import FasitLogo from '../lib/icons/fasit'


const LogoBox = styled.div`
  cursor: pointer;
  width: 100px;
  margin-right: 12px;
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
