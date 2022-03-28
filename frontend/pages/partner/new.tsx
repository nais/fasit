import * as React from 'react'
import Head from 'next/head'
import { NewPartnerForm } from '../../components/partner/newPartnerForm'

const NewPartner = () => {
  return (
    <>
      <Head>
        <title>Ny partner</title>
      </Head>
      <NewPartnerForm />
    </>
  )
}

export default NewPartner