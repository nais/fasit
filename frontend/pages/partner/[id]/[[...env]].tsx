import { useRouter } from 'next/router'
import * as React from 'react'
import { useState } from 'react'
import { useEnvironmentsGetQuery, usePartnerGetQuery } from '../../../lib/schema/graphql'
import ErrorMessage from '../../../components/lib/error'
import LoaderSpinner from '../../../components/lib/spinner'
import AddEnvironment from '../../../components/partner/addEnvironment'
import { GetServerSideProps } from 'next'
import { addApolloState, initializeApollo } from '../../../lib/apollo'
import { PARTNER_GET } from '../../../lib/queries/partner/partnerGet'
import Environment from '../../../components/partner/environment'
import { Main, MenuItem, MenuItems, MenuSeparator, PageContainer, SideMenu } from '../../../components/lib/PageLayout'


const Partner = () => {
  const router = useRouter()
  const partnerID = router.query.id as string
  const envID = router.query.env as string


  const { data, error, loading } = usePartnerGetQuery({ variables: { id: partnerID } })
  const envQuery = useEnvironmentsGetQuery({ variables: { partnerID } })
  const [open, setOpen] = useState(false)

  if (error) {
    return <ErrorMessage error={error} />
  }
  if (loading || !data?.partner) {
    return <LoaderSpinner />
  }
  const partner = data.partner

  return (
    <PageContainer>
      <SideMenu>
        {envQuery.loading || !envQuery.data && <LoaderSpinner />}
        {envQuery.error && <ErrorMessage error={envQuery.error} />}
        <MenuItems>
          {envQuery.data?.environments?.map((e, i) => {
            return (
              <MenuItem onClick={() => router.push(`/partner/${partnerID}/${e.id}`)} key={`${e.name}_${i}`} active={e.id == envID}>
                <a>{e.name}</a>
              </MenuItem>
            )
          })}
          {envQuery.data && envQuery.data.environments?.length > 0 && <MenuSeparator />}
          <MenuItem onClick={() => setOpen(true)}><a>Nytt miljø</a></MenuItem>
        </MenuItems>
      </SideMenu>
      <Main>
        {envID && <Environment envID={envID[0]} partnerName={partner.name} />}
      </Main>
      <AddEnvironment open={open} onClose={() => setOpen(false)} partnerName={partner.name}
        partnerID={partner.id} />
    </PageContainer>
  )

}
export const getServerSideProps: GetServerSideProps = async (context) => {
  const { id } = context.query

  const apolloClient = initializeApollo()

  try {
    await apolloClient.query({
      query: PARTNER_GET,
      variables: { id },
    })
  } catch (e) {
    console.log(e)
  }

  return addApolloState(apolloClient, {
    props: { id },
  })
}
export default Partner
