import * as React from 'react'
import Head from 'next/head'
import { NewTenantForm } from '../../components/tenant/newTenantForm'

const NewTenant = () => {
  return (
    <>
      <Head>
        <title>New tenant</title>
      </Head>
      <NewTenantForm />
    </>
  )
}

export default NewTenant