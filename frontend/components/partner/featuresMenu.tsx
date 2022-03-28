import * as React from 'react'
import { useFeaturesQuery } from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import Link from 'next/link'
import { useRouter } from 'next/router'
import styled from 'styled-components'
import { navRod } from '../../styles/constants'

const SideMenu = styled.div`
  padding: 10px 0px 10px 10px;
  display: flex;
  flex-direction: column;
  border-top: 1px solid silver;
  border-left: 1px solid silver;
  border-bottom: 1px solid silver;
  border-radius: 5px 0px 0px 5px;
  background-color: #f5f5f5;
  height: fit-content;
  border-right: 1px solid #fff;
  position: relative;
`

interface MenuItemProps {
  active?: boolean
}

const MenuItem = styled.div<MenuItemProps>`
  ${(props) => props.active && 'background-color: #fff;'}
  border-top: 1px solid ${(props) => props.active ? 'silver' : 'transparent'};
  border-bottom: 1px solid ${(props) => props.active ? 'silver' : 'transparent'};
  border-left: 3px solid ${(props) => props.active ? navRod : 'transparent'};
  border-radius: 5px 0px 0px 5px;
  padding: 5px 15px;
  margin-right: -2px;
  position: relative;
  * {
    text-decoration: none;
    color: #222;
  }
  :hover {
    background-color: var(--navds-semantic-color-interaction-primary-hover-subtle);
  }

`

const FeaturesMenu = () => {
  const features = useFeaturesQuery()
  const { data, loading, error } = features
  const router = useRouter()
  const feature = router.query.feature

  if (error) return <ErrorMessage error={error} />
  if (!data || loading) return <LoaderSpinner />

  return (<SideMenu>
    <i style={{ marginBottom: "15px" }}>Features</i>

    {features.data && features.data.features.map((f) => {
      return <MenuItem key={f.name} active={f.name === feature}>
        <Link href={router.asPath.split('?')[0] + '?feature=' + f.name}>
          <a>{f.name}</a>
        </Link></MenuItem>
    })}

  </SideMenu>
  )
}
export default FeaturesMenu