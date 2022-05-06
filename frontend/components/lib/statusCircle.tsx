import styled from "styled-components";

interface ReportCircleProps {
    color: string
}

const StatusCircle = styled.span<ReportCircleProps>`
  width: 0.75em;
  height: 0.75em;
  border-radius: 50%;
  display: inline-block;
  vertical-align: middle;
  margin-left: 5px;
  background-color: ${(props) => props.color};
`
export default StatusCircle