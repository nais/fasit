import styled from 'styled-components'
import React, { useContext } from 'react'
import IconButton from '@mui/material/IconButton'
import { AddCircleFilled } from '@navikt/ds-icons'
import { useRouter } from 'next/router'

const CreateBox = styled.div`
  white-space: nowrap;
  display: flex;
  align-items: center;
  margin-left: auto;
  margin-right: 10px;
`

const CreateTenant = () => {
  const router = useRouter()

  return (
    <CreateBox>
      <IconButton
        size='large'
        edge='end'
        aria-label='Add new tenant'
        aria-controls='primary-search-account-menu'
        aria-haspopup='true'
        onClick={async () => await router.push('/tenant/new')}
        color='inherit'
      >
        <AddCircleFilled />
      </IconButton>
    </CreateBox>
  )
}

export default CreateTenant
