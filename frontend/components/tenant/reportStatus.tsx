import * as React from 'react'
import styled from 'styled-components'
import humanizeDate from "../lib/humanizeDate";
import {differenceInMinutes, parseISO} from "date-fns";
import {navGronn, navOransje, navRod} from "../../styles/constants";
import {useEffect, useState} from "react";

interface ReportCircleProps {
    color: string
}

const ReportCircle = styled.span<ReportCircleProps>`
  width: 0.75em;
  height: 0.75em;
  border-radius: 50%;
  display: inline-block;
  vertical-align: middle;
  margin-left: 5px;
  background-color: ${(props) => props.color};
`
const reportStatus = (date: string) => {
    const distance = differenceInMinutes(Date.now(), parseISO(date))
    return <ReportCircle color={distance > 4 ? navRod : distance > 1 ? navOransje : navGronn}/>
}

interface NaisdProps {
    reportedAt: string
}

const ReportStatus = ({reportedAt}: NaisdProps) => {
    const [time, setTime] = useState(Date.now());
    useEffect(() => {
        const interval = setInterval(() => setTime(Date.now()), 10 * 1000)
        return () => {
            clearInterval(interval);
        };
    }, [])
    return <span>
        <b>Last seen:</b>
        {humanizeDate(reportedAt, "", true)}
        {reportStatus(reportedAt)}
    </span>
}
export default ReportStatus