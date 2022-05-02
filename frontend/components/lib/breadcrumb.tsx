import {useRouter} from "next/router";
import styled from "styled-components";

const TenantHeaderName = styled.h1`
  text-transform: capitalize;
  color: #222;
  margin: 0px;
`
const BreadCrumb = () => {
    const router = useRouter()
    const path = router.asPath
    console.log(router.asPath.split("/"))
    return (
        <div>{path.split("/")[2]}</div>
    )
}
export default BreadCrumb