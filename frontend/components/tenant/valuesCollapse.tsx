import React from 'react';
import useCollapse from 'react-collapsed';
import styled from "styled-components";
import {Expand, Collapse} from "@navikt/ds-icons";
import {MappingValue} from "../../lib/schema/graphql";
import ReactTooltip from "react-tooltip";

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

const isJson = (x: any) => {
    try {
        JSON.parse(x);
    } catch (e) {
        return false;
    }
    return true;
}

function safeToString(x: any) {
    if (isJson(x)) {
        return JSON.stringify(x, null, 2)
    }
    if (x.toString() === '[object Object]') {
        return JSON.stringify(x, null, 2)
    }
    return x.toString()
}

const ValuesCollapse = ({content}: DropdownProps) => {
    const {getCollapseProps, getToggleProps, isExpanded} = useCollapse();
    return (
        Array.isArray(content.value) && content.value.length > 1 ?
            <div className="collapsible">
                <Header {...getToggleProps()}>
                    {isExpanded ?
                        <>
                            <Collapse data-tip data-for={"collapse"}/>
                            <ReactTooltip id='collapse' place='top' type='dark' effect='solid'> Collapse </ReactTooltip>
                        </> :
                        <>
                            <Expand data-tip data-for={"expand"}/>
                            <ReactTooltip id='expand' place='top' type='dark' effect='solid'> Expand </ReactTooltip>
                        </>}
                </Header>
                <div {...getCollapseProps()}>
                    <LogPre>{JSON.stringify(content.value, null, 2)}</LogPre>
                </div>
            </div>
            : safeToString(content.value)
    );
}
export default ValuesCollapse
