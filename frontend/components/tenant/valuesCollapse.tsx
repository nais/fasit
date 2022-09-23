import React from 'react';
import useCollapse from 'react-collapsed';
import styled from "styled-components";
import {MappingValue} from "../../lib/schema/graphql";

const LogPre = styled.pre`
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 12px;
`

const Header = styled.div`
    cursor: pointer;
    text-align: left;
`

interface DropdownProps {
    content: MappingValue
}

const isJson = (str: string) => {
    try {
        JSON.parse(str);
    } catch (e) {
        return false;
    }
    return true;
}

const ValuesCollapse = ({content}: DropdownProps) => {
    const {getCollapseProps, getToggleProps, isExpanded} = useCollapse();
    return (
        Array.isArray(content.value) ?
            <div className="collapsible">
                <Header {...getToggleProps()}>
                    {isExpanded ? '-' : '+'}
                </Header>
                <div {...getCollapseProps()}>
                    <LogPre>{JSON.stringify(content.value, null, 2)}</LogPre>
                </div>
            </div>
            : isJson(content.value) ? JSON.stringify(content.value) : content.value.toString()
    );
}
export default ValuesCollapse
