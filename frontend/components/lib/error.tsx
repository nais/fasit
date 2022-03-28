import styled from 'styled-components'
import { navGraBakgrunn, navRod } from '../../styles/constants'
import { ErrorFilled } from '@navikt/ds-icons'

const ErrorMessageDiv = styled.div`
  background-color: ${navGraBakgrunn};
  div {
    display: flex;
    align-items: center;
    font-size: 24px;
  }
  p {
    margin: 0 5px 5px 29px;
  }
  border-radius: 5px;
  padding: 5px 10px;
  svg {
    margin-right: 5px;
    color: ${navRod};
  }
`

interface errorMessageProps {
    error: Error
}

export const ErrorMessage = ({ error }: errorMessageProps) => {
    return (
        <ErrorMessageDiv>
            <div>
                <ErrorFilled />
                Feil
            </div>
            <p>{error.message}</p>
        </ErrorMessageDiv>
    )
}

export default ErrorMessage