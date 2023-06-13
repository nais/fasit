import { useRouter } from 'next/router'
import styled from 'styled-components'
import { Home } from '@navikt/ds-icons'
import Link from 'next/link'
import { navRod } from '../../styles/constants'

styled(Link)``
const BreadCrumbBox = styled.div`
  display: flex;
  a {
    color: #222;
    text-decoration: none;
    text-transform: uppercase;

    margin: 0 2px;
    :hover {
      color: ${navRod};
    }
  }
  a:not(:last-child)::after {
    color: #aaa;
    margin-left: 10px;
    content: '>';
  }
`
const BreadCrumb = () => {
  const router = useRouter()
  const path = router.asPath.split('/')

  return (
    <BreadCrumbBox>
      <Link href={'/'}>
        <Home />
      </Link>

      {path.slice(2).map((p, index) => {
        return p.includes('?') ? (
          <Link
            key={p}
            href={
              path
                .slice(0, index + 3)
                .join('/')
                .split('?')[0]
            }
          >
            {p.split('?')[0]}
          </Link>
        ) : (
          <Link key={p} href={path.slice(0, index + 3).join('/')}>
            {p}
          </Link>
        )
      })}
    </BreadCrumbBox>
  )
}
export default BreadCrumb
