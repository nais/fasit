import * as React from 'react'
import {EnvironmentGetQuery, useFeaturesQuery} from '../../lib/schema/graphql'
import ErrorMessage from '../lib/error'
import LoaderSpinner from '../lib/spinner'
import Link from 'next/link'
import {useRouter} from 'next/router'
import styled from 'styled-components'
import {navRod} from '../../styles/constants'

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
  enabled?: boolean
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
  a {
    text-decoration: none;
    color: ${(props) =>  props.enabled ? '#222' : "#999"};
  }
  :hover {
    background-color: var(--navds-semantic-color-interaction-primary-hover-subtle);
  }

`
interface FeaturesMenuProps {
  env: EnvironmentGetQuery['environment']

}
const FeaturesMenu = ({env}: FeaturesMenuProps) => {
  const features = useFeaturesQuery()
  const { data, loading, error } = features
  const router = useRouter()
  const feature = router.query.feature

  if (error) return <ErrorMessage error={error} />
  if (!data || loading) return <LoaderSpinner />

  return (<SideMenu>
    <i style={{ marginBottom: "15px" }}>Features</i>

    {env.featureStates.filter((f) => f.enabled).map((fs) => {
      const f = fs.feature
      return <MenuItem key={f.name} active={f.name === feature} enabled={fs.enabled}>

        <Link href={router.asPath.split('?')[0] + '?feature=' + f.name}>
          <a>{f.name}</a>
        </Link></MenuItem>
    })}
        <hr style={{width: '100%'}}/>
        {env.featureStates.filter((f) => !f.enabled).map((fs) => {
          const f = fs.feature
          return <MenuItem key={f.name} active={f.name === feature} enabled={fs.enabled}>

            <Link href={router.asPath.split('?')[0] + '?feature=' + f.name}>
              <a>{f.name}</a>
            </Link></MenuItem>
        })}

  </SideMenu>
  )
}
export default FeaturesMenu