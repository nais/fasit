import NavLogo from "./icons/navLogo";
import TenantLogo from "./icons/tenantLogo";

const GetLogo = (name: string, size?: number) => {
    switch (name) {
        case 'nav':
            return <NavLogo />
        case 'mattilsynet':
        case 'mtpilot':
            return <img src={"/mattilsynet.png"} width={size?size+'px':'50px'}/>
        default:
            return <TenantLogo />

    }
}
export default GetLogo