import { Box, Modal } from '@mui/material'
import * as React from 'react'
import { useState } from 'react'
import ErrorMessage from '../lib/error'
import RightJustifiedSubmitButton from '../lib/submitButton'
import { useFeatureStateSaveMutation } from '../../lib/schema/graphql'

const style = {
  position: 'absolute' as 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  width: 500,
  bgcolor: 'background.paper',
  border: '2px solid #000',
  boxShadow: 24,
  p: 4,
}

interface RedeployFeatureProps {
  open: boolean
  onClose: React.Dispatch<boolean>
  feature: string
  envID: string
}

const RedeployFeature = ({
  open,
  onClose,
  feature,
  envID,
}: RedeployFeatureProps) => {
  const [backendError, setBackendError] = useState()
  const [save] = useFeatureStateSaveMutation()

  const onSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    try {
      await save({
        variables: { feature, enabled: true, envID },
        awaitRefetchQueries: true,
        refetchQueries: ['environmentGetByNames'],
        onCompleted: () => onClose(false),
        onError: (e) => console.log(e),
      })
    } catch (e: any) {
      console.log(e)
      setBackendError(e)
    }
  }

  return (
    <Modal open={open} onClose={() => onClose(false)}>
      <Box sx={style}>
        {backendError && <ErrorMessage error={backendError} />}
        <form onSubmit={onSubmit}>
          <p style={{ padding: '0px 0px 30px 0px' }}>
            Are you sure you want to redeploy {feature}?
          </p>
          <RightJustifiedSubmitButton onCancel={() => onClose(false)} />
        </form>
      </Box>
    </Modal>
  )
}
export default RedeployFeature
