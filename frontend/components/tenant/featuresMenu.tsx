import Link from 'next/link'
import { useRouter } from 'next/router'
import styled from 'styled-components'
import {
  EnvironmentGetByNamesQuery,
  RolloutStatus,
} from '../../lib/schema/graphql'
import { navRod } from '../../styles/constants'

const SideMenu = styled.div<MenuItemProps>`
  padding: 10px 0px 10px 10px;
  display: flex;
  flex-direction: column;
  border-top: 1px solid silver;
  border-left: 1px solid silver;
  border-bottom: 1px solid silver;
  border-radius: 5px 0px 0px 5px;
  background-color: #f5f5f5;
  height: fit-content;
  min-width: 150px;
  position: relative;
  a {
    text-decoration: none;
  }
`

interface MenuItemProps {
  active?: boolean
  enabled?: boolean
  failed?: boolean
}

const MenuItem = styled.div<MenuItemProps>`
  ${(props) => props.active && 'background-color: #fff;'}
  border-top: 1px solid ${(props) => (props.active ? 'silver' : 'transparent')};
  border-bottom: 1px solid
    ${(props) => (props.active ? 'silver' : 'transparent')};
  border-left: 3px solid ${(props) => (props.active ? navRod : 'transparent')};
  border-radius: 5px 0px 0px 5px;
  padding: 5px 15px;
  margin-right: ${(props) => (props.active ? '-1px' : '0px')};
  position: relative;
  text-decoration: none;
  color: ${(props) => (props.failed ? 'red' : props.enabled ? '#222' : '#999')};
  :hover {
    background-color: var(
      --navds-semantic-color-interaction-primary-hover-subtle
    );
  }
`

export const MenuSeparator = styled.div`
  border-bottom: 1px solid #c0c0c0;
  margin: 10px 0;
  margin-left: -10px;
`

interface FeaturesMenuProps {
  env: EnvironmentGetByNamesQuery['environmentByNames']
}

const FeaturesMenu = ({ env }: FeaturesMenuProps) => {
  const router = useRouter()
  const feature = router.query.feature

  const featureMenuItem = (
    fs: EnvironmentGetByNamesQuery['environmentByNames']['featureStates'][0],
  ) => {
    const f = fs.feature
    return (
      <Link
        href={router.asPath.split('?')[0] + '?feature=' + f.name}
        key={f.name}
      >
        <a>
          <MenuItem
            active={f.name === feature}
            enabled={fs.enabled}
            failed={fs.rolloutStatus === RolloutStatus.Failed}
          >
            {f.name}
          </MenuItem>
        </a>
      </Link>
    )
  }

  return (
    <SideMenu>
      <span style={{ marginBottom: '15px' }}>Features</span>

      {env.featureStates.filter((f) => f.enabled).map(featureMenuItem)}
      <MenuSeparator />
      {env.featureStates.filter((f) => !f.enabled).map(featureMenuItem)}
    </SideMenu>
  )
}
export default FeaturesMenu
