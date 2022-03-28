import NavLogo from "./icons/navLogo";
import PartnerLogo from "./icons/partner";

const GetLogo = (name: string, size?: number) => {
    switch (name) {
        case 'nav':
            return <NavLogo />
        case 'mattilsynet':
            return <img src={"/mattilsynet.png"} width={size?size+'px':'50px'}/>
        default:
            return <PartnerLogo />

    }
}
export default GetLogo