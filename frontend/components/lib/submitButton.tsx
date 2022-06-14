import { Button, Loader } from '@navikt/ds-react'
import styled from 'styled-components'

export const SubmitButton = styled.div`
  width: 100%;
  display: flex;
  justify-content: flex-end;

  & button {
    margin: 1em 0 1em 1em;
  }
`

interface RightJustifiedSubmitButtonProps {
    onCancel?: () => void
    loading?: boolean
}

export const RightJustifiedSubmitButton = ({
                                               onCancel,
                                               loading,
                                           }: RightJustifiedSubmitButtonProps) => (
    <SubmitButton>
        <Button type={'submit'}>
            {loading ? (
                <>
                    <Loader /> Saving...
                </>
            ) : (
                'Save'
            )}
        </Button>
        {onCancel && (
            <Button type={'button'} variant={'danger'} onClick={onCancel}>
                Cancel
            </Button>
        )}
    </SubmitButton>
)

export default RightJustifiedSubmitButton