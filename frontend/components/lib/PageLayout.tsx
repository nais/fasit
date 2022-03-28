import styled from "styled-components";
import {navRod} from "../../styles/constants";

interface SideMenuProps {
    width?: number
}
export const SideMenu = styled.div<SideMenuProps>`
  min-width: ${(props) => props.width ? `${props.width}px`: '150px'};
  max-width: ${(props) => props.width ? `${props.width}px`: '150px'};
  border-right: 1px solid #c0c0c0;
  overflow-wrap: break-word;
  margin-right: 15px;
  height: fit-content;
`
export const Main = styled.div`
  margin: 30px 0 0 20px;
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  > div > h1 {margin-top: 0px;}
`
export const MenuSeparator = styled.div`
  border-bottom: 1px solid #c0c0c0;
  margin: 10px 0;
`
export const MenuItems = styled.div`
  display: flex;
  flex-direction: column;
  padding-top: 30px;
  font-size: 1.1em;
`

export const PageContainer = styled.div`
  display: flex;
`

interface MenuItemProps {
    active?: boolean
}

export const MenuItem = styled.div<MenuItemProps>`
  border-left: ${(props) => props.active ? `5px solid ${navRod};` : '5px solid transparent;'}
  padding: 5px 10px;
  transition: border-left 0.3s ease-in-out;
  cursor: pointer;
  * {
    text-decoration: none;
    text-transform: uppercase;
    color:rgba(0, 0, 0, 0.6) ;
  }
  :hover {
    background-color: var(--navds-semantic-color-interaction-primary-hover-subtle);
  }
`

